package netutil

import (
	"net"
	"strings"

	"hvc/pkg/logx"
)

// DetectAdvertiseIP 自动检测本机局域网 IP 地址。
//
// 检测策略：
//  1. 通过 UDP Dial 尝试连接外部地址（不实际发包），获取本机出口 IP
//  2. 如果 UDP 方式失败，遍历本机网卡，选择第一个非回环的 IPv4 地址
//  3. 如果都失败，返回 127.0.0.1
//
// 此方法适用于局域网环境，无需手动配置 advertise_ip。
func DetectAdvertiseIP() string {
	if ip := detectByUDP(); ip != "" {
		return ip
	}
	if ip := detectByInterfaces(); ip != "" {
		return ip
	}
	logx.Info("netutil.advertise_ip.fallback_to_localhost", nil)
	return "127.0.0.1"
}

// detectByUDP 通过 UDP Dial 检测本机出口 IP。
//
// 向一个公共 DNS 地址发起 UDP 连接（不实际发包），
// 操作系统会自动选择出口网卡，从而获取本机在该网卡上的 IP。
func detectByUDP() string {
	conn, err := net.Dial("udp4", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	ip := addr.IP.String()
	if ip != "" && !strings.HasPrefix(ip, "127.") {
		logx.Info("netutil.advertise_ip.detected_by_udp", logx.Fields{
			"ip": ip,
		})
		return ip
	}
	return ""
}

// detectByInterfaces 遍历本机网卡，选择第一个非回环的 IPv4 地址。
func detectByInterfaces() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
				continue
			}
			ip := ipNet.IP.String()
			if strings.HasPrefix(ip, "169.254.") {
				continue
			}
			logx.Info("netutil.advertise_ip.detected_by_interface", logx.Fields{
				"ip":    ip,
				"iface": iface.Name,
			})
			return ip
		}
	}
	return ""
}
