package utils

import (
	"net"
	"testing"
)

func TestGetHostIP(t *testing.T) {
	ip, err := GetHostIP()
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}

	if ip == "" {
		t.Error("GetHostIP returned empty string")
	}

	// Verify it's a valid IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		t.Errorf("GetHostIP returned invalid IP: %s", ip)
	}

	// Verify it's an IPv4 address
	if parsedIP.To4() == nil {
		t.Errorf("GetHostIP returned non-IPv4 address: %s", ip)
	}
}