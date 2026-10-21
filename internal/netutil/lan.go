// Package netutil contains the IP/subnet primitives behind the LAN gate.
package netutil

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// ParseClientIP parses an address as seen by the server, normalising the
// forms that otherwise defeat CIDR checks:
//   - "host:port" and "[v6]:port"
//   - IPv6 zone suffixes ("fe80::1%eth0")
//   - IPv4-mapped IPv6 ("::ffff:192.168.1.7") -> 4-byte IPv4
func ParseClientIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	if i := strings.IndexByte(s, '%'); i >= 0 {
		s = s[:i]
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// InAny reports whether ip lies in any of nets. A nil ip is never inside.
func InAny(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// Interface is one usable LAN interface address.
type Interface struct {
	Name string
	IP   net.IP
	Net  *net.IPNet
}

// LANInterfaces lists up, non-loopback, private (RFC1918/ULA) unicast IPv4
// and IPv6 addresses — the networks an exam hall can sit on. Link-local
// and public addresses are excluded.
func LANInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || !ipn.IP.IsPrivate() {
				continue
			}
			ip := ipn.IP
			if v4 := ip.To4(); v4 != nil {
				ip = v4
			}
			_, network, err := net.ParseCIDR(ipn.String())
			if err != nil {
				continue
			}
			out = append(out, Interface{Name: iface.Name, IP: ip, Net: network})
		}
	}
	// IPv4 first: student devices reach "http://192.168.x.y" far more easily
	// than a bracketed IPv6 literal.
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].IP.To4() != nil && out[i].IP.To4() == nil {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// ErrNoLAN means no private interface was found and none was configured.
var ErrNoLAN = errors.New("no private LAN interface found; set LAN_CIDRS and LAN_ADVERTISE_IP")

// AdvertiseIP returns the address students should type: the configured one,
// else the first private IPv4 interface.
func AdvertiseIP(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	ifs, err := LANInterfaces()
	if err != nil {
		return "", err
	}
	if len(ifs) == 0 {
		return "", ErrNoLAN
	}
	return ifs[0].IP.String(), nil
}

// DetectLANNets returns the networks of every private interface.
func DetectLANNets() ([]*net.IPNet, error) {
	ifs, err := LANInterfaces()
	if err != nil {
		return nil, err
	}
	nets := make([]*net.IPNet, 0, len(ifs))
	for _, i := range ifs {
		nets = append(nets, i.Net)
	}
	if len(nets) == 0 {
		return nil, ErrNoLAN
	}
	return nets, nil
}

// HostURL formats scheme://ip[:port], bracketing IPv6 literals and omitting
// the scheme's default port.
func HostURL(scheme, ip string, port int) string {
	host := ip
	if strings.Contains(ip, ":") {
		host = "[" + ip + "]"
	}
	if port == 0 || (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return scheme + "://" + host
	}
	return scheme + "://" + net.JoinHostPort(ip, strconv.Itoa(port))
}
