package models

type Switch struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	IPAddress string
	Community string
	System    string // "generic" or "unifi"
}

type PortStatus struct {
	ID            uint `gorm:"primaryKey"`
	SwitchID      uint
	PortIndex     int
	PortName      string
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
