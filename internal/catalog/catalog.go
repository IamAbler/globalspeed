package catalog

import (
	"embed"

	"fmt"
	"net"
	"strconv"
	"strings"
)

//go:embed serverlist_encrypt.json tests.json
var assets embed.FS

type Server struct {
	CityID   string `json:"cityid"`
	GeoID    string `json:"geoid"`
	ID       string `json:"hostid"`
	IP       string `json:"hostip"`
	Name     string `json:"hostname"`
	City     string `json:"location"`
	Operator string `json:"oper"`
	Province string `json:"pname"`
	Port     string `json:"port"`
}

func Servers(province, operator string) []Server {
	all := defaultStore.servers()
	out := make([]Server, 0, len(all))
	for _, s := range all {
		if (province == "" || strings.Contains(s.Province, province)) && (operator == "" || s.Operator == operator) {
			out = append(out, s)
		}
	}
	return out
}
func Find(id string) (Server, error) {
	for _, s := range Servers("", "") {
		if s.ID == id {
			return s, nil
		}
	}
	return Server{}, fmt.Errorf("节点 %q 不存在", id)
}
func (s Server) Address() (string, error) {
	ip := net.ParseIP(s.IP)
	port, err := strconv.Atoi(s.Port)
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("节点地址无效")
	}
	return net.JoinHostPort(s.IP, s.Port), nil
}
