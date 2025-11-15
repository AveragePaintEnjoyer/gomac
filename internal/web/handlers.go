package web

import (
	"go-mac/internal/db"
	"go-mac/internal/models"
	"go-mac/internal/portname"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		var switches []models.Switch
		db.DB.Find(&switches)

		// Collect site names
		siteSet := map[string]struct{}{}
		for _, sw := range switches {
			if sw.Site == "" {
				sw.Site = "default"
			}
			siteSet[sw.Site] = struct{}{}
		}

		var sites []string
		for s := range siteSet {
			sites = append(sites, s)
		}

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

		for _, sw := range switches {
			var ports []models.PortStatus
			allowedTypes := []string{"ethernetCsmacd", "gigabitEthernet"}

			db.DB.Where("switch_id = ? AND if_type IN ?", sw.ID, allowedTypes).
				Order("port_index asc").
				Find(&ports)

			var switchPorts []PortView
			for _, p := range ports {
				var macs []models.MacEntry
				db.DB.Where("switch_id = ? AND port_index = ?", sw.ID, p.PortIndex).Find(&macs)

				displayName := portname.Normalize(p.PortName)

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
			"Sites":    sites,
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
			Site:      c.FormValue("site"),
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
