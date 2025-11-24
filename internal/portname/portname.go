package portname

import "regexp"

type Rule struct {
	Regex   *regexp.Regexp
	Handler func(match []string) string
}

var Rules = []Rule{
	// Slot/Port style
	{regexp.MustCompile(`Slot:\s*\d+\s*Port:\s*(\d+)`),
		func(m []string) string { return m[1] }},

	// Generic "Port16" or "Port: 16"
	{regexp.MustCompile(`Port\s*:?(\d+)`),
		func(m []string) string { return m[1] }},

	// SFP ports → "s1"
	{regexp.MustCompile(`SFP\+?(\d+)`),
		func(m []string) string { return "s" + m[1] }},

	// Cisco long 3-level
	{regexp.MustCompile(`(?i)(?:GigabitEthernet|TenGigabitEthernet|FastEthernet)\s*\d+/\d+/(\d+)`),
		func(m []string) string { return m[1] }},

	// Cisco long 2-level
	{regexp.MustCompile(`(?i)(?:GigabitEthernet|TenGigabitEthernet|FastEthernet)\s*\d+/(\d+)`),
		func(m []string) string { return m[1] }},

	// Cisco long 1-level
	{regexp.MustCompile(`(?i)(?:GigabitEthernet|TenGigabitEthernet|FastEthernet)\s*(\d+)$`),
		func(m []string) string { return m[1] }},

	// Cisco short (Gi, Te, Fa)
	{regexp.MustCompile(`(?i)(?:Gi|Te|Fa)\s*\d+/\d+/(\d+)`),
		func(m []string) string { return m[1] }},
	{regexp.MustCompile(`(?i)(?:Gi|Te|Fa)\s*\d+/(\d+)`),
		func(m []string) string { return m[1] }},
	{regexp.MustCompile(`(?i)(?:Gi|Te|Fa)\s*(\d+)$`),
		func(m []string) string { return m[1] }},

	// Huawei (GE, XGE)
	{regexp.MustCompile(`(?i)(?:GE|XGE)\s*\d+/\d+/(\d+)`),
		func(m []string) string { return m[1] }},

	// Juniper (ge-0/0/1, xe-1/2/3)
	{regexp.MustCompile(`(?i)(?:ge|xe|et)-\d+/\d+/(\d+)`),
		func(m []string) string { return m[1] }},
}

// Normalize extracts a short, consistent label.
func Normalize(name string) string {
	for _, r := range Rules {
		if m := r.Regex.FindStringSubmatch(name); len(m) > 1 {
			return r.Handler(m)
		}
	}
	return name
}
