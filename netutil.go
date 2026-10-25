package main

import "net"

func getLocalIPv4s() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					ips = append(ips, ip4.String())
				}
			}
		}
	}
	if len(ips) == 0 {
		return []string{"127.0.0.1"}
	}
	return ips
}

func getBroadcastIPs() []string {
	seen := map[string]bool{"255.255.255.255": true}
	var list []string
	list = append(list, "255.255.255.255")

	ifaces, err := net.Interfaces()
	if err != nil {
		return list
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip4 := ipnet.IP.To4()
				mask := ipnet.Mask
				if ip4 != nil && len(mask) == 4 {
					bcast := net.IPv4(
						ip4[0]|^mask[0],
						ip4[1]|^mask[1],
						ip4[2]|^mask[2],
						ip4[3]|^mask[3],
					).String()
					if !seen[bcast] {
						seen[bcast] = true
						list = append(list, bcast)
					}
				}
			}
		}
	}
	return list
}
