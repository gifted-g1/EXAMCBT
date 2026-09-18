package exam

import (
	"encoding/base64"
	"fmt"
	"net"

	qrcode "github.com/skip2/go-qrcode"
)

// LANServerInfo describes the locally reachable exam server that
// students on the same Wi-Fi/LAN connect to during a LAN-mode exam.
type LANServerInfo struct {
	NetworkInterface string `json:"network_interface"`
	IPAddress        string `json:"ip_address"`
	Port             int    `json:"port"`
	AccessURL        string `json:"access_url"`
	QRCodeBase64PNG  string `json:"qr_code_base64_png"`
}

// DetectLocalIP finds the machine's outward-facing LAN IP by picking
// the first non-loopback IPv4 address bound to an active interface.
// This intentionally avoids hard-coding any IP: it reflects whatever
// network the admin's machine is actually connected to.
func DetectLocalIP() (iface string, ip string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}

	for _, i := range ifaces {
		// Skip interfaces that are down or loopback.
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var candidate net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				candidate = v.IP
			case *net.IPAddr:
				candidate = v.IP
			}
			if candidate == nil || candidate.IsLoopback() {
				continue
			}
			v4 := candidate.To4()
			if v4 == nil {
				continue // prefer IPv4 for broad student-device compatibility
			}
			return i.Name, v4.String(), nil
		}
	}
	return "", "", fmt.Errorf("no active non-loopback network interface found")
}

// FindAvailablePort scans the configured port range and returns the
// first port the process can successfully bind, so the exam server
// never collides with another service already running on the machine.
func FindAvailablePort(host string, start, end int) (int, error) {
	for port := start; port <= end; port++ {
		addr := fmt.Sprintf("%s:%d", host, port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", start, end)
}

// BuildLANServerInfo detects the LAN IP, picks a free port, and
// generates a scannable QR code encoding the student access URL.
func BuildLANServerInfo(portRangeStart, portRangeEnd int) (*LANServerInfo, error) {
	iface, ip, err := DetectLocalIP()
	if err != nil {
		return nil, err
	}
	port, err := FindAvailablePort(ip, portRangeStart, portRangeEnd)
	if err != nil {
		return nil, err
	}
	accessURL := fmt.Sprintf("http://%s:%d", ip, port)

	png, err := qrcode.Encode(accessURL, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}

	return &LANServerInfo{
		NetworkInterface: iface,
		IPAddress:        ip,
		Port:             port,
		AccessURL:        accessURL,
		QRCodeBase64PNG:  base64.StdEncoding.EncodeToString(png),
	}, nil
}
