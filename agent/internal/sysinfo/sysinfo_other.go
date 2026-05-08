//go:build !windows

// Package sysinfo exposes lightweight system metadata used to populate the
// per-agent info row in the Skreen dashboard (hostname, username, idle time).
package sysinfo

import (
	"io"
	"net/http"
	"os/user"
	"strings"
	"time"
)

// FetchPublicIP retrieves the agent's public IP from a remote service.
func FetchPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://icanhazip.com")
	if err != nil {
		return "Unknown"
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(body))
}

var (
	cachedPublicIP    string
	lastPublicIPFetch time.Time
)

func getCachedPublicIP() string {
	if cachedPublicIP != "" && time.Since(lastPublicIPFetch) < 10*time.Minute {
		return cachedPublicIP
	}
	cachedPublicIP = FetchPublicIP()
	lastPublicIPFetch = time.Now()
	return cachedPublicIP
}

// GetIdleSeconds returns 0 on non-Windows platforms.
func GetIdleSeconds() int64 {
	return 0
}

// GetUsername returns the currently logged-in username.
func GetUsername() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}

// GetSystemStats returns stub stats on non-Windows platforms.
func GetSystemStats() SystemStats {
	return SystemStats{
		CPU:      "Linux/Generic",
		LocalIP:  "127.0.0.1",
		PublicIP: getCachedPublicIP(),
	}
}
