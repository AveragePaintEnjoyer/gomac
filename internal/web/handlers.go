package web

import (
	"go-mac/internal/db"
	"go-mac/internal/models"
	"go-mac/internal/poller"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gosnmp/gosnmp"
)

func SetupRoutes(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		var switches []models.Switch
		db.DB.Find(&switches)

		type PortView struct {
			Index         int
			Status        string
			StatusChanges int
			Macs          []models.MacEntry
			SwitchID      uint
		}

		type SwitchView struct {
			Switch models.Switch
			Ports  []PortView
		}

		var viewData []SwitchView

		for _, sw := range switches {
			var ports []models.PortStatus
			db.DB.Where("switch_id = ?", sw.ID).Order("port_index asc").Find(&ports)

			var switchPorts []PortView
			for _, p := range ports {
				var macs []models.MacEntry
				db.DB.Where("switch_id = ? AND port_index = ?", sw.ID, p.PortIndex).Find(&macs)

				switchPorts = append(switchPorts, PortView{
					Index:    p.PortIndex,
					Status:   p.Status,
					Macs:     macs,
					SwitchID: sw.ID,
				})
			}

			viewData = append(viewData, SwitchView{
				Switch: sw,
				Ports:  switchPorts,
			})
		}

		return c.Render("index", fiber.Map{
			"Switches": viewData,
		})
	})

	// MAC search page (GET displays form, POST performs search)
	app.Get("/mac", func(c *fiber.Ctx) error {
		return c.Render("mac", fiber.Map{
			"Results": nil,
			"Query":   "",
		})
	})

	app.Post("/mac", func(c *fiber.Ctx) error {
		query := c.FormValue("mac") // get MAC search input

		if query == "" {
			return c.Redirect("/mac")
		}

		type Result struct {
			MAC       string
			Switch    string
			IP        string
			PortIndex int
			VLAN      int
		}

		var results []Result

		// Perform a case-insensitive search in the MAC table
		db.DB.Table("mac_entries").
			Select("mac_entries.mac, switches.name as switch, switches.ip_address as ip, mac_entries.port_index, mac_entries.vlan").
			Joins("left join switches on switches.id = mac_entries.switch_id").
			Where("mac_entries.mac LIKE ?", "%"+query+"%").
			Scan(&results)

		return c.Render("mac", fiber.Map{
			"Results": results,
			"Query":   query,
		})
	})

	// Show the test form
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.Render("test", fiber.Map{
			"Result": nil,
		})
	})

	app.Post("/test", func(c *fiber.Ctx) error {
		ip := c.FormValue("ip")
		community := c.FormValue("community")
		system := c.FormValue("system")
		portCountStr := c.FormValue("portcount")
		portCount, _ := strconv.Atoi(portCountStr)

		type PortResult struct {
			Index         int
			Status        string
			StatusChanges int
			Macs          []struct {
				MAC  string
				VLAN int
			}
		}

		var results []PortResult

		// --- SNMP Port Status ---
		ports, err := poller.SnmpInterfaceWalk(ip, community, portCount)
		if err == nil {
			for i := 1; i <= portCount; i++ {
				status := ports[i]
				results = append(results, PortResult{
					Index:  i,
					Status: status,
				})
			}
		}

		// --- SNMP MAC Table ---
		macWalk, err := poller.MacSNMPWalk(ip, community, system)
		if err == nil {
			for _, p := range results {
				p.Macs = []struct {
					MAC  string
					VLAN int
				}{}
			}

			for _, mac := range macWalk {
				port := int(gosnmp.ToBigInt(mac.Value).Int64())
				macOID := strings.TrimPrefix(mac.Name, ".")
				vlan := poller.ExtractVLAN(macOID, system)
				macDec := poller.OidTrimmer(macOID, system)
				if system == "unifi" {
					parts := strings.Split(macDec, ".")
					if len(parts) > 1 {
						macDec = strings.Join(parts[1:], ".")
					}
				}
				macHex := poller.DeciMacToHex(macDec)

				if macHex != "" && port >= 1 && port <= portCount {
					results[port-1].Macs = append(results[port-1].Macs, struct {
						MAC  string
						VLAN int
					}{MAC: macHex, VLAN: vlan})
				}
			}
		}

		return c.Render("test", fiber.Map{
			"Results":   results,
			"IP":        ip,
			"Community": community,
			"System":    system,
			"PortCount": portCount,
		})
	})

	// Admin form
	app.Get("/admin", func(c *fiber.Ctx) error {
		var switches []models.Switch
		db.DB.Find(&switches)
		return c.Render("admin", fiber.Map{
			"Switches": switches,
		})
	})

	// Handle form submit
	app.Post("/admin/add", func(c *fiber.Ctx) error {
		portCount, _ := strconv.Atoi(c.FormValue("portcount"))
		s := models.Switch{
			Name:      c.FormValue("name"),
			IPAddress: c.FormValue("ip"),
			Community: c.FormValue("community"),
			System:    c.FormValue("system"),
			PortCount: portCount,
		}
		db.DB.Create(&s)
		return c.Redirect("/admin")
	})

	// Delete a switch
	app.Post("/admin/delete/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		db.DB.Delete(&models.Switch{}, id)
		return c.Redirect("/admin")
	})
}
