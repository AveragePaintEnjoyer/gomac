package web

import (
	"regexp"

	"go-mac/internal/db"
	"go-mac/internal/models"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		var switches []models.Switch
		db.DB.Find(&switches)

		type PortView struct {
			Index         int
			Name          string
			DisplayName   string
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

		// Precompiled regexes for speed
		reSlotPort := regexp.MustCompile(`Slot:\s*\d+\s*Port:\s*(\d+)`)
		rePortNum := regexp.MustCompile(`Port\s*:?(\d+)`)
		reSFP := regexp.MustCompile(`SFP\+\d+`)
		reSFPNum := regexp.MustCompile(`SFP\+?(\d+)`)
		reCisco := regexp.MustCompile(`(?:GigabitEthernet|FastEthernet|TenGigabitEthernet)\d+\/(\d+)`)

		for _, sw := range switches {
			var ports []models.PortStatus
			// Only load Ethernet ports
			db.DB.Where("switch_id = ? AND if_type = ?", sw.ID, "ethernet-csmacd").
				Order("port_index asc").
				Find(&ports)

			var switchPorts []PortView
			for _, p := range ports {
				var macs []models.MacEntry
				db.DB.Where("switch_id = ? AND port_index = ?", sw.ID, p.PortIndex).Find(&macs)

				displayName := p.PortName // default

				switch {
				case reSlotPort.MatchString(p.PortName):
					match := reSlotPort.FindStringSubmatch(p.PortName)
					displayName = match[1]
				case rePortNum.MatchString(p.PortName):
					match := rePortNum.FindStringSubmatch(p.PortName)
					displayName = match[1]
				case reSFP.MatchString(p.PortName):
					if reSFPNum.MatchString(p.PortName) {
						num := reSFPNum.FindStringSubmatch(p.PortName)[1]
						displayName = "s" + num
					} else {
						displayName = "sfp"
					}
				case reCisco.MatchString(p.PortName):
					displayName = reCisco.FindStringSubmatch(p.PortName)[1]
				}

				switchPorts = append(switchPorts, PortView{
					Index:         p.PortIndex,
					Name:          p.PortName,
					DisplayName:   displayName,
					Status:        p.Status,
					StatusChanges: p.StatusChanges,
					Macs:          macs,
					SwitchID:      sw.ID,
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
		s := models.Switch{
			Name:      c.FormValue("name"),
			IPAddress: c.FormValue("ip"),
			Community: c.FormValue("community"),
			System:    c.FormValue("system"),
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
