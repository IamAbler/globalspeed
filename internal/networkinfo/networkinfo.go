package networkinfo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const Endpoint = "https://myip.ipip.net/json"

type Info struct {
	IP       string `json:"ip"`
	Operator string `json:"operator"`
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	Location string `json:"location"`
}

// Lookup reports the public address visible to IPIP, rather than a LAN address.
func Lookup(ctx context.Context, client *http.Client, endpoint string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Info{}, errors.New("公网 IP 获取失败")
	}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, errors.New("公网 IP 获取失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, errors.New("公网 IP 服务响应错误")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16385))
	if err != nil || len(data) > 16384 {
		return Info{}, errors.New("公网 IP 服务响应错误")
	}
	var response struct {
		Ret  string `json:"ret"`
		Data struct {
			IP       string   `json:"ip"`
			Location []string `json:"location"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &response) != nil || response.Ret != "ok" || net.ParseIP(response.Data.IP) == nil {
		return Info{}, errors.New("公网 IP 服务响应错误")
	}
	result := Info{IP: response.Data.IP}
	fields := []*string{&result.Country, &result.Province, &result.City}
	for i, field := range fields {
		if i < len(response.Data.Location) {
			*field = strings.TrimSpace(response.Data.Location[i])
		}
	}
	if len(response.Data.Location) > 4 {
		result.Operator = strings.TrimSpace(response.Data.Location[4])
	}
	var parts []string
	for _, field := range fields {
		if *field != "" && (len(parts) == 0 || parts[len(parts)-1] != *field) {
			parts = append(parts, *field)
		}
	}
	result.Location = strings.Join(parts, " · ")
	return result, nil
}
