package poller

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var operState = map[int]string{
	1: "UP",
	2: "DOWN",
	3: "TESTING",
	4: "UNKNOWN",
	5: "DORMANT",
	6: "NOT_PRESENT",
	7: "LOWER_LAYER_DOWN",
}

type Switch struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	IPAddress string
	Community string
	System    string
	PortCount int
}

type PortStatus struct {
	ID            uint `gorm:"primaryKey"`
	SwitchID      uint
	PortIndex     int
	Status        string
	StatusChanges int
}

type MacEntry struct {
	ID        uint `gorm:"primaryKey"`
	SwitchID  uint
	PortIndex int
	VLAN      int
	MAC       string
}

// ---------- SNMP FUNCTIONS ----------

func SnmpInterfaceWalk(host, community string, ifaceCount int) (map[int]string, error) {
	oid := "1.3.6.1.2.1.2.2.1.8"

	g := &gosnmp.GoSNMP{
		Target:    host,
		Port:      161,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   gosnmp.Default.Timeout,
		Retries:   1,
	}

	err := g.Connect()
	if err != nil {
		return nil, fmt.Errorf("connect error: %v", err)
	}
	defer g.Conn.Close()

	ifaces := make(map[int]string)

	err = g.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		// Extract interface index from OID
		oidStr := strings.TrimPrefix(pdu.Name, ".")
		idxStr := strings.TrimPrefix(oidStr, "1.3.6.1.2.1.2.2.1.8.")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil // skip invalid
		}

		// Skip ports outside the relevant interface count
		if idx < 1 || idx > ifaceCount {
			return nil
		}

		// Get oper status value
		val := int(gosnmp.ToBigInt(pdu.Value).Int64())
		state, ok := operState[val]
		if !ok {
			state = fmt.Sprintf("UNKNOWN(%d)", val)
		}

		ifaces[idx] = state
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("SNMP walk error: %v", err)
	}

	return ifaces, nil
}

func MacSNMPWalk(host, community, system string) ([]gosnmp.SnmpPDU, error) {
	var macOID string
	if system == "unifi" {
		// This table lists MACs per VLAN context
		macOID = "1.3.6.1.2.1.17.7.1.2.2.1.2"
	} else {
		macOID = "1.3.6.1.2.1.17.4.3.1.2"
	}

	g := &gosnmp.GoSNMP{
		Target:    host,
		Port:      161,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   gosnmp.Default.Timeout,
		Retries:   1,
	}
	if err := g.Connect(); err != nil {
		return nil, err
	}
	defer g.Conn.Close()

	var results []gosnmp.SnmpPDU
	err := g.BulkWalk(macOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})
	return results, err
}

func OidTrimmer(oid, system string) string {
	if system == "unifi" {
		return strings.TrimPrefix(oid, "1.3.6.1.2.1.17.7.1.2.2.1.2.")
	}
	return strings.TrimPrefix(oid, "1.3.6.1.2.1.17.4.3.1.2.")
}

func DeciMacToHex(macAddress string) string {
	parts := strings.Split(macAddress, ".")
	if len(parts) != 6 {
		return ""
	}
	hexParts := make([]string, 6)
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		hexParts[i] = fmt.Sprintf("%02x", n)
	}
	return strings.Join(hexParts, ":")
}

func ExtractVLAN(oid, system string) int {
	if system != "unifi" {
		return 0
	}

	parts := strings.Split(oid, ".")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "2" && i+3 < len(parts) &&
			parts[i+1] == "2" && parts[i+2] == "1" && parts[i+3] == "2" {
			if i+4 < len(parts) {
				vlan, _ := strconv.Atoi(parts[i+4])
				return vlan
			}
		}
	}
	return 0
}

// ---------- POLLER LOOP ----------

func pollSwitch(db *gorm.DB, sw Switch) {
	fmt.Printf("Polling %s (%s)\n", sw.Name, sw.IPAddress)

	// --- PORT STATUS ---
	ports, err := SnmpInterfaceWalk(sw.IPAddress, sw.Community, sw.PortCount)
	if err == nil {
		for idx, newState := range ports {
			var existing PortStatus
			tx := db.Where("switch_id = ? AND port_index = ?", sw.ID, idx).First(&existing)
			if tx.Error == nil {
				if existing.Status != newState {
					existing.Status = newState
					existing.StatusChanges++
					db.Save(&existing)
				}
			} else {
				db.Create(&PortStatus{SwitchID: sw.ID, PortIndex: idx, Status: newState, StatusChanges: 0})
			}
		}
	}

	// --- MAC TABLE ---
	macWalk, err := MacSNMPWalk(sw.IPAddress, sw.Community, sw.System)
	if err == nil {
		for _, mac := range macWalk {
			port := int(gosnmp.ToBigInt(mac.Value).Int64())
			macOID := strings.TrimPrefix(mac.Name, ".")
			vlan := ExtractVLAN(macOID, sw.System)

			macDec := OidTrimmer(macOID, sw.System)
			if sw.System == "unifi" {
				// Remove VLAN part from OID to get MAC bytes
				parts := strings.Split(macDec, ".")
				if len(parts) > 1 {
					macDec = strings.Join(parts[1:], ".")
				}
			}
			macHex := DeciMacToHex(macDec)

			if macHex != "" {
				var count int64
				db.Model(&MacEntry{}).
					Where("switch_id = ? AND port_index = ? AND vlan = ? AND mac = ?", sw.ID, port, vlan, macHex).
					Count(&count)
				if count == 0 {
					db.Create(&MacEntry{SwitchID: sw.ID, PortIndex: port, VLAN: vlan, MAC: macHex})
				}
			}
		}
	}
}

// ---------- MAIN LOOP ----------

func StartBackgroundPolling(interval time.Duration, path string) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&Switch{}, &PortStatus{}, &MacEntry{})

	for {
		var switches []Switch
		db.Find(&switches)

		for _, sw := range switches {
			pollSwitch(db, sw)
		}

		fmt.Println("Polling cycle complete.")
		time.Sleep(interval)
	}
}
