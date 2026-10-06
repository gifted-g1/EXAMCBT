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

// BuildLANServerInfo detects the host's real LAN IP and builds the
// access URL/QR code around the port this same backend process is
// already listening on (servePort — the app's own HTTP_PORT).
//
// IMPORTANT: earlier versions of this function scanned a separate
// port range and returned whichever port happened to be free at that
// instant, without ever binding a server to it — the QR code pointed
// students at a dead port. The backend now serves both the REST/WS
// API *and* the built frontend (see internal/webui) from one process
// on one port, so the LAN access URL is simply that same port,
// reachable at the host's real LAN IP instead of localhost.
func BuildLANServerInfo(servePort int) (*LANServerInfo, error) {
	iface, ip, err := DetectLocalIP()
	if err != nil {
		return nil, err
	}
	accessURL := fmt.Sprintf("http://%s:%d", ip, servePort)

	png, err := qrcode.Encode(accessURL, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}

	return &LANServerInfo{
		NetworkInterface: iface,
		IPAddress:        ip,
		Port:             servePort,
		AccessURL:        accessURL,
		QRCodeBase64PNG:  base64.StdEncoding.EncodeToString(png),
	}, nil
}
