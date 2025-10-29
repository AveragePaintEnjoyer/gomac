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

	"go-mac/internal/models"
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

// ---------- SNMP FUNCTIONS ----------

func SnmpInterfaces(host, community string) (map[int]string, map[int]string, error) {
	ifDescrOID := "1.3.6.1.2.1.2.2.1.2"
	ifOperOID := "1.3.6.1.2.1.2.2.1.8"

	g := &gosnmp.GoSNMP{
		Target:         host,
		Port:           161,
		Community:      community,
		Version:        gosnmp.Version2c,
		Timeout:        5 * time.Second,
		Retries:        2,
		MaxRepetitions: 50,
	}
	if err := g.Connect(); err != nil {
		return nil, nil, fmt.Errorf("SNMP connect failed: %v", err)
	}
	defer g.Conn.Close()

	descrs := make(map[int]string)
	if err := g.BulkWalk(ifDescrOID, func(pdu gosnmp.SnmpPDU) error {
		oidClean := strings.TrimPrefix(pdu.Name, ".") // remove leading dot
		idxStr := strings.TrimPrefix(oidClean, ifDescrOID+".")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			fmt.Printf("Skipping OID %s, cannot parse index: %v\n", pdu.Name, err)
			return nil
		}

		var name string
		switch v := pdu.Value.(type) {
		case []byte:
			name = strings.TrimSpace(string(v))
		case string:
			name = strings.TrimSpace(v)
		default:
			name = fmt.Sprintf("%v", v)
		}

		descrs[idx] = name
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("ifDescr walk error: %v", err)
	}

	statuses := make(map[int]string)
	if err := g.BulkWalk(ifOperOID, func(pdu gosnmp.SnmpPDU) error {
		// Extract interface index from OID
		idxStr := strings.TrimPrefix(pdu.Name, ".")        // remove leading dot
		idxStr = strings.TrimPrefix(idxStr, ifOperOID+".") // remove OID prefix
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			fmt.Printf("Skipping OID %s, cannot parse index: %v\n", pdu.Name, err)
			return nil
		}

		// Convert SNMP value to integer
		val := 0
		switch v := pdu.Value.(type) {
		case int:
			val = v
		case uint:
			val = int(v)
		case int64:
			val = int(v)
		case uint64:
			val = int(v)
		default:
			val = int(gosnmp.ToBigInt(pdu.Value).Int64())
		}

		state, ok := operState[val]
		if !ok {
			state = fmt.Sprintf("UNKNOWN(%d)", val)
		}
		statuses[idx] = state
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("ifOperStatus walk error: %v", err)
	}

	return descrs, statuses, nil
}

func MacSNMPWalk(host, community, system string) ([]gosnmp.SnmpPDU, error) {
	var macOID string
	if system == "unifi" {
		macOID = "1.3.6.1.2.1.17.7.1.2.2.1.2"
	} else {
		macOID = "1.3.6.1.2.1.17.4.3.1.2"
	}

	g := &gosnmp.GoSNMP{
		Target:         host,
		Port:           161,
		Community:      community,
		Version:        gosnmp.Version2c,
		Timeout:        5 * time.Second,
		Retries:        2,
		MaxRepetitions: 50,
	}
	if err := g.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed for MAC walk: %v", err)
	}
	defer g.Conn.Close()

	var results []gosnmp.SnmpPDU
	if err := g.BulkWalk(macOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("MAC walk error: %v", err)
	}

	return results, nil
}

// ---------- UTILS ----------

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

// ---------- POLLING ----------

func pollSwitch(db *gorm.DB, sw models.Switch) {
	fmt.Printf("Polling switch %s (%s)\n", sw.Name, sw.IPAddress)

	descrs, statuses, err := SnmpInterfaces(sw.IPAddress, sw.Community)
	if err != nil {
		fmt.Printf("Error polling interfaces: %v\n", err)
		return
	}

	if len(descrs) == 0 {
		fmt.Println("No interfaces found!")
		return
	}

	for idx, name := range descrs {
		status, ok := statuses[idx]
		if !ok {
			status = "UNKNOWN"
		}

		var iface models.PortStatus
		tx := db.Where("switch_id = ? AND port_index = ?", sw.ID, idx).First(&iface)
		if tx.Error == nil {
			updated := false
			if iface.Status != status {
				iface.Status = status
				iface.StatusChanges++
				updated = true
			}
			if iface.PortName != name {
				iface.PortName = name
				updated = true
			}
			if updated {
				db.Save(&iface)
			}
		} else if tx.Error == gorm.ErrRecordNotFound {
			db.Create(&models.PortStatus{
				SwitchID:  sw.ID,
				PortIndex: idx,
				PortName:  name,
				Status:    status,
			})
		}
	}

	// --- MAC TABLE ---
	macWalk, err := MacSNMPWalk(sw.IPAddress, sw.Community, sw.System)
	if err != nil {
		fmt.Printf("MAC walk error: %v\n", err)
		return
	}

	for _, mac := range macWalk {
		port := int(gosnmp.ToBigInt(mac.Value).Int64())
		macOID := strings.TrimPrefix(mac.Name, ".")
		vlan := ExtractVLAN(macOID, sw.System)

		macDec := OidTrimmer(macOID, sw.System)
		if sw.System == "unifi" {
			parts := strings.Split(macDec, ".")
			if len(parts) > 1 {
				macDec = strings.Join(parts[1:], ".")
			}
		}
		macHex := DeciMacToHex(macDec)
		if macHex == "" {
			continue
		}

		var count int64
		db.Model(&models.MacEntry{}).
			Where("switch_id = ? AND port_index = ? AND vlan = ? AND mac = ?", sw.ID, port, vlan, macHex).
			Count(&count)
		if count == 0 {
			db.Create(&models.MacEntry{
				SwitchID:  sw.ID,
				PortIndex: port,
				VLAN:      vlan,
				MAC:       macHex,
			})
		}
	}
}

// ---------- MAIN LOOP ----------

func StartBackgroundPolling(interval time.Duration, path string) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Switch{}, &models.PortStatus{}, &models.MacEntry{}); err != nil {
		log.Fatal("Failed to migrate DB:", err)
	}

	for {
		var switches []models.Switch
		db.Find(&switches)

		if len(switches) == 0 {
			fmt.Println("No switches found in DB.")
		}

		for _, sw := range switches {
			pollSwitch(db, sw)
		}

		fmt.Println("Polling cycle complete.")
		time.Sleep(interval)
	}
}
