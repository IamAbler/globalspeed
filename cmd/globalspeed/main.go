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
		fmt.Println("GlobalSpeed · 本机直连\n\n  globalspeed nodes [--province 浙江] [--operator 电信] [--json]\n  globalspeed speed --server 5 [--duration 5] [--connections 2] [--max-mib 64] [--json] [--no-history]\n  globalspeed dns www.baidu.com\n  globalspeed history [--path]\n\nCtrl+C 停止测速，自动释放节点会话。流量预算不包含 HTTP/TCP 头部。")
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
		duration := f.Int("duration", 5, "每阶段秒数")
		connections := f.Int("connections", 2, "并发数")
		maxMiB := f.Int("max-mib", 64, "总负载预算 MiB")
		asJSON := f.Bool("json", false, "JSON 输出")
		noHistory := f.Bool("no-history", false, "不保存本地历史")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if *id == "" {
			return errors.New("请用 --server 指定节点 ID")
		}
		options := speed.Options{ServerID: *id, DurationSeconds: *duration, Connections: *connections, MaxMiB: *maxMiB}
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
		fmt.Printf("%s · %s\nHTTP 延迟 %.2f ms · 抖动 %.2f ms\n下载 %.2f Mbps · 上传 %.2f Mbps\n流量 %.2f MiB · 会话已释放 %t\n", result.Server.Name, result.Path, *result.HTTPMedianMS, *result.HTTPJitterMS, result.Download.Mbps, result.Upload.Mbps, float64(result.Download.Bytes+result.Upload.Bytes)/float64(speed.MiB), result.Released)
		return nil
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
