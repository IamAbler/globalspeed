package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"globalspeed/internal/catalog"
	"globalspeed/internal/history"
	"globalspeed/internal/speed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
func emit(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("GlobalSpeed · 本机直连\n\n  globalspeed nodes [--province 浙江] [--operator 电信] [--json]\n  globalspeed speed [--auto | --server 5] [--duration 5] [--connections 2] [--max-mib 64] [--json] [--no-history]\n  globalspeed select [--province 江苏] [--operator 电信]\n  globalspeed dns www.baidu.com\n  globalspeed history [--path]\n\nCtrl+C 停止测速，自动释放节点会话。流量预算不包含 HTTP/TCP 头部。")
		return nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	switch args[0] {
	case "nodes":
		f := flag.NewFlagSet("nodes", flag.ContinueOnError)
		province := f.String("province", "", "省份")
		operator := f.String("operator", "", "运营商")
		asJSON := f.Bool("json", false, "JSON 输出")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		all := catalog.Servers(*province, *operator)
		if *asJSON {
			return emit(all)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\t省份\t运营商\t节点\t地址")
		for _, s := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s:%s\n", s.ID, s.Province, s.Operator, s.Name, s.IP, s.Port)
		}
		return w.Flush()
	case "speed":
		f := flag.NewFlagSet("speed", flag.ContinueOnError)
		id := f.String("server", "", "节点 ID（nodes 命令查询）")
		auto := f.Bool("auto", false, "按原版流程自动选点")
		province := f.String("province", "", "自动选点省份")
		city := f.String("city", "", "自动选点城市")
		operator := f.String("operator", "", "自动选点运营商")
		publicIP := f.String("ip", "", "自动选点公网 IP（可留空）")
		network := f.Int("network", 5, "原版选点网络参数 4/5")
		duration := f.Int("duration", 5, "每阶段秒数")
		connections := f.Int("connections", 2, "并发数")
		maxMiB := f.Int("max-mib", 64, "总负载预算 MiB")
		asJSON := f.Bool("json", false, "JSON 输出")
		noHistory := f.Bool("no-history", false, "不保存本地历史")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if *id == "" && !*auto {
			return errors.New("请用 --auto 或 --server 指定节点")
		}
		if *auto && *id != "" {
			return errors.New("--auto 与 --server 不能同时使用")
		}
		options := speed.Options{Auto: *auto, Match: speed.MatchOptions{Province: *province, City: *city, Operator: *operator, IP: *publicIP, Network: *network}, ServerID: *id, DurationSeconds: *duration, Connections: *connections, MaxMiB: *maxMiB}
		var progress func(speed.Progress)
		if !*asJSON {
			progress = func(p speed.Progress) {
				fmt.Fprintf(os.Stderr, "\r%-10s %8.2f Mbps · %.2f MiB    ", p.Phase, p.Mbps, float64(p.Bytes)/float64(speed.MiB))
			}
		}
		result, err := speed.NewClient().Run(ctx, options, progress)
		if !*asJSON {
			fmt.Fprintln(os.Stderr)
		}
		if err != nil {
			return err
		}
		if !*noHistory {
			if e := history.Save(result); e != nil {
				fmt.Fprintln(os.Stderr, "历史保存失败:", e)
			}
		}
		if *asJSON {
			return emit(result)
		}
		fmt.Printf("%s · %s\n%s\n下载 %.2f Mbps · 上传 %.2f Mbps\n流量 %.2f MiB · 会话已释放 %t\n", result.Server.Name, result.Path, result.Ping.Description(), result.Download.Mbps, result.Upload.Mbps, float64(result.Download.Bytes+result.Upload.Bytes)/float64(speed.MiB), result.Released)
		return nil
	case "select":
		f := flag.NewFlagSet("select", flag.ContinueOnError)
		province := f.String("province", "", "省份")
		city := f.String("city", "", "城市")
		operator := f.String("operator", "", "运营商")
		publicIP := f.String("ip", "", "公网 IP（可留空）")
		network := f.Int("network", 5, "原版网络参数 4/5")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		server, err := speed.NewClient().Match(ctx, speed.MatchOptions{Province: *province, City: *city, Operator: *operator, IP: *publicIP, Network: *network}, nil)
		if err != nil {
			return err
		}
		return emit(server)
	case "dns":
		if len(args) != 2 || strings.ContainsAny(args[1], " /\\") {
			return errors.New("用法: globalspeed dns 域名")
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, args[1])
		if err != nil {
			return err
		}
		return emit(ips)
	case "history":
		if len(args) == 2 && args[1] == "--path" {
			p, err := history.Path()
			if err != nil {
				return err
			}
			fmt.Println(p)
			return nil
		}
		if len(args) != 1 {
			return errors.New("用法: globalspeed history [--path]")
		}
		all, err := history.Load()
		if err != nil {
			return err
		}
		return emit(all)
	default:
		return fmt.Errorf("未知命令 %q；运行 globalspeed help 查看用法", args[0])
	}
}
