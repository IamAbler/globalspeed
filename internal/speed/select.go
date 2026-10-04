package speed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"globalspeed/internal/catalog"
	"globalspeed/internal/ping"
	"io"
	"net/http"
	"net/url"
	"time"
)

const matchURL = "https://dlcv2.cnspeedtest.cn:8443/dataServer/mobilematch_many.php"

type MatchOptions struct {
	IP             string `json:"ip"`
	Province       string `json:"province"`
	City           string `json:"city"`
	Operator       string `json:"operator"`
	MobileOperator string `json:"mobileOperator"`
	Network        int    `json:"network"`
}

func (c *Client) Match(ctx context.Context, o MatchOptions, progress func(Progress)) (catalog.Server, error) {
	return c.match(ctx, o, matchURL, func(ctx context.Context, s catalog.Server) (ping.Result, error) {
		address, err := s.Address()
		if err != nil {
			return ping.Result{}, err
		}
		return ping.TCP(ctx, address, 2, time.Second)
	}, progress)
}
func (c *Client) match(ctx context.Context, o MatchOptions, endpoint string, probe func(context.Context, catalog.Server) (ping.Result, error), progress func(Progress)) (catalog.Server, error) {
	if o.Network == 0 {
		o.Network = 5
	}
	if o.Network != 4 && o.Network != 5 {
		return catalog.Server{}, errors.New("network 必须为 4 或 5")
	}
	if progress != nil {
		progress(Progress{Phase: "selecting"})
	}
	q := url.Values{"ip": {o.IP}, "network": {fmt.Sprint(o.Network)}, "province": {o.Province}, "city": {o.City}, "wifioper": {o.Operator}, "mobileoperid": {o.MobileOperator}, "ipv6": {"0"}, "model": {"Android"}, "pkg": {"com.cnspeedtest.globalspeed"}}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return catalog.Server{}, err
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return catalog.Server{}, ctx.Err()
		}
		return catalog.Server{}, errors.New("自动选点服务请求失败（APK 状态 121）")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return catalog.Server{}, fmt.Errorf("自动选点服务 HTTP %d（APK 状态 122）", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if ctx.Err() != nil {
		return catalog.Server{}, ctx.Err()
	}
	if err != nil || len(data) > 1024*1024 {
		return catalog.Server{}, errors.New("自动选点目录响应无效")
	}
	var rows []struct {
		catalog.Server
		MatchCity string `json:"city"`
	}
	if err = json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return catalog.Server{}, errors.New("自动选点目录为空或无效（APK 状态 122）")
	}
	candidates := make([]catalog.Server, 0, len(rows))
	for _, row := range rows {
		s := row.Server
		s.City = row.MatchCity
		if s.ID == "" || s.Name == "" {
			return catalog.Server{}, errors.New("自动选点节点格式无效")
		}
		if _, err := s.Address(); err != nil {
			return catalog.Server{}, err
		}
		candidates = append(candidates, s)
	}
	// ServerMatchTask preserves vendor order; it does not choose lowest RTT.
	selected := candidates[0]
	for _, s := range candidates {
		if ctx.Err() != nil {
			return catalog.Server{}, ctx.Err()
		}
		measured, err := probe(ctx, s)
		if ctx.Err() != nil {
			return catalog.Server{}, ctx.Err()
		}
		if err == nil && measured.Status == 0 && measured.AverageMS > .1 {
			selected = s
			break
		}
	}
	if progress != nil {
		progress(Progress{Phase: "selected", Server: &selected})
	}
	return selected, nil
}
