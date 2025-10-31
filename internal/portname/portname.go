package portname

import "regexp"

type Rule struct {
	Regex   *regexp.Regexp
	Handler func(match []string) string
}

var Rules = []Rule{
	// Slot/Port style: "Slot: 0 Port: 2 Gigabit - Level"
	{
		regexp.MustCompile(`Slot:\s*\d+\s*Port:\s*(\d+)`),
		func(m []string) string { return m[1] },
	},
	// Generic "Port16" or "Port: 16"
	{
		regexp.MustCompile(`Port\s*:?(\d+)`),
		func(m []string) string { return m[1] },
	},
	// SFP ports like "SFP+1" → "s1"
	{
		regexp.MustCompile(`SFP\+?(\d+)`),
		func(m []string) string { return "s" + m[1] },
	},
	// Cisco-style 3-level: "GigabitEthernet1/0/48" → "48"
	{
		regexp.MustCompile(`(?:GigabitEthernet|TenGigabitEthernet|FastEthernet)\d+/\d+/(\d+)`),
		func(m []string) string { return m[1] },
	},
	// Cisco-style 2-level: "GigabitEthernet0/9" → "9"
	{
		regexp.MustCompile(`(?:GigabitEthernet|TenGigabitEthernet|FastEthernet)\d+/(\d+)`),
		func(m []string) string { return m[1] },
	},
}

// Normalize extracts a short, consistent label for display on port boxes.
func Normalize(name string) string {
	for _, rule := range Rules {
		if match := rule.Regex.FindStringSubmatch(name); len(match) > 1 {
			return rule.Handler(match)
		}
	}
	return name
}
