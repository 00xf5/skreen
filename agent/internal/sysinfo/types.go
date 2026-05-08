package sysinfo

// SystemStats represents the hardware and OS telemetry for an agent.
type SystemStats struct {
	CPU         string `json:"cpu"`
	RAMTotal    uint64 `json:"ram_total"`
	RAMUsed     uint64 `json:"ram_used"`
	DiskTotal   uint64 `json:"disk_total"`
	DiskFree    uint64 `json:"disk_free"`
	Uptime      uint64 `json:"uptime"`
	LocalIP     string `json:"local_ip"`
	PublicIP    string `json:"public_ip"`
}
