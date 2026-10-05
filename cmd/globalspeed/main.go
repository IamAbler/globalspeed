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

	"github.com/mattn/go-isatty"
	"globalspeed/internal/catalog"
	"globalspeed/internal/history"
	"globalspeed/internal/networkinfo"
	"globalspeed/internal/speed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "[error]", err)
		os.Exit(1)
	}
}
func emit(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
func run(args []string) error {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Println("GlobalSpeed · 本机直连\n\n  globalspeed [--server-id 5] [--format human-readable|json] [--progress yes|no]\n  globalspeed nodes [--province 浙江] [--operator 电信] [--json]\n  globalspeed speed [--auto | --server 5] [--duration 5] [--connections 2] [--max-mib 64] [--json] [--no-history]\n  globalspeed update [--json]\n  globalspeed nodes --path\n  globalspeed select [--province 江苏] [--operator 电信]\n  globalspeed dns www.baidu.com\n  globalspeed history [--path]\n\nCtrl+C 停止测速，自动释放节点会话。流量预算不包含 HTTP/TCP 头部。")
		return nil
	}
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-V") {
		fmt.Println("GlobalSpeed 0.1.0")
		return nil
	}
	if len(args) == 0 {
		args = []string{"speed"}
	} else if strings.HasPrefix(args[0], "-") {
		args = append([]string{"speed"}, args...)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	switch args[0] {
	case "nodes":
		f := flag.NewFlagSet("nodes", flag.ContinueOnError)
		province := f.String("province", "", "省份")
		operator := f.String("operator", "", "运营商")
		asJSON := f.Bool("json", false, "JSON 输出")
		noUpdate := f.Bool("no-update", false, "使用本地目录，不检查更新")
		showPath := f.Bool("path", false, "显示加密目录路径")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if *showPath {
			path, err := catalog.Path()
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		}
		if !*noUpdate {
			refreshCatalog(ctx)
		}
		if ctx.Err() != nil {
			return speed.PublicError(ctx.Err())
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
		f.StringVar(id, "server-id", "", "节点 ID")
		f.StringVar(id, "s", "", "节点 ID")
		format := f.String("format", "human-readable", "输出格式：human-readable / json / json-pretty")
		f.StringVar(format, "f", "human-readable", "输出格式")
		progressMode := f.String("progress", "auto", "实时进度：yes / no（默认仅终端显示）")
		f.StringVar(progressMode, "p", "auto", "实时进度：yes / no")
		noProgress := f.Bool("no-progress", false, "禁用实时进度")
		auto := f.Bool("auto", false, "自动选择节点")
		province := f.String("province", "", "自动选点省份")
		city := f.String("city", "", "自动选点城市")
		operator := f.String("operator", "", "自动选点运营商")
		publicIP := f.String("ip", "", "自动选点公网 IP（可留空）")
		network := f.Int("network", 5, "选点网络参数 4/5")
		duration := f.Int("duration", 5, "每阶段秒数")
		connections := f.Int("connections", 2, "并发数")
		maxMiB := f.Int("max-mib", 64, "总负载预算 MiB")
		asJSON := f.Bool("json", false, "JSON 输出")
		noHistory := f.Bool("no-history", false, "不保存本地历史")
		noUpdate := f.Bool("no-update", false, "使用本地目录，不检查更新")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return errors.New("不支持额外位置参数；运行 globalspeed --help 查看用法")
		}
		if *auto && *id != "" {
			return errors.New("--auto 与 --server 不能同时使用")
		}
		if *format != "human-readable" && *format != "json" && *format != "json-pretty" {
			return errors.New("--format 必须为 human-readable、json 或 json-pretty")
		}
		if *progressMode != "auto" && *progressMode != "yes" && *progressMode != "no" {
			return errors.New("--progress 必须为 yes 或 no")
		}
		jsonOutput := *asJSON || *format != "human-readable"
		options := speed.Options{Auto: *id == "", Match: speed.MatchOptions{Province: *province, City: *city, Operator: *operator, IP: *publicIP, Network: *network}, ServerID: *id, DurationSeconds: *duration, Connections: *connections, MaxMiB: *maxMiB}
		if err := options.Validate(); err != nil {
			return err
		}
		if *network != 4 && *network != 5 {
			return speed.Failure(136, nil)
		}
		if !*noUpdate {
			refreshCatalog(ctx)
		}
		if ctx.Err() != nil {
			return speed.PublicError(ctx.Err())
		}
		if !options.Auto {
			if _, err := catalog.Find(options.ServerID); err != nil {
				return speed.Failure(139, err)
			}
		}
		p := presentation{out: os.Stdout, live: !jsonOutput && !*noProgress && *progressMode != "no" && (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())), duration: *duration, budget: int64(*maxMiB) * speed.MiB / 2}
		defer p.clear()
		if !jsonOutput {
			p.header()
			p.status("   Retrieving network information...")
		}
		client := speed.NewClient()
		var info *networkinfo.Info
		found, infoErr := networkinfo.Lookup(ctx, client.HTTP, networkinfo.Endpoint)
		if ctx.Err() != nil {
			return speed.PublicError(ctx.Err())
		}
		if infoErr == nil {
			info = &found
			if options.Match.IP == "" {
				options.Match.IP = found.IP
			}
			if options.Match.Province == "" {
				options.Match.Province = found.Province
			}
			if options.Match.City == "" && (*province == "" || *province == found.Province) {
				options.Match.City = found.City
			}
			if options.Match.Operator == "" {
				options.Match.Operator = found.Operator
			}
		}
		var server catalog.Server
		var err error
		if options.Auto {
			if !jsonOutput {
				p.status("   Selecting server...")
			}
			server, err = client.Match(ctx, options.Match, nil)
		} else {
			server, err = catalog.Find(options.ServerID)
		}
		if err != nil {
			return speed.PublicError(err)
		}
		if !jsonOutput {
			p.server(server, info)
		}
		var progress func(speed.Progress)
		if !jsonOutput {
			progress = p.progress
		}
		result, err := client.RunSelected(ctx, options, server, progress)
		p.clear()
		if err != nil {
			return err
		}
		if !*noHistory {
			if e := history.Save(result); e != nil {
				fmt.Fprintln(os.Stderr, "[warning] 历史保存失败:", e)
			}
		}
		if jsonOutput {
			if *format == "json" && !*asJSON {
				return json.NewEncoder(os.Stdout).Encode(result)
			}
			return emit(result)
		}
		p.complete(result)
		return nil
	case "update":
		f := flag.NewFlagSet("update", flag.ContinueOnError)
		asJSON := f.Bool("json", false, "JSON 输出")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		result, err := catalog.Update(ctx)
		if ctx.Err() != nil {
			return speed.PublicError(ctx.Err())
		}
		if err != nil {
			return err
		}
		if *asJSON {
			return emit(result)
		}
		fmt.Printf("节点目录已检查：%d 个节点\n%s\n", result.Count, result.Path)
		return nil
	case "select":
		f := flag.NewFlagSet("select", flag.ContinueOnError)
		province := f.String("province", "", "省份")
		city := f.String("city", "", "城市")
		operator := f.String("operator", "", "运营商")
		publicIP := f.String("ip", "", "公网 IP（可留空）")
		network := f.Int("network", 5, "选点网络参数 4/5")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		refreshCatalog(ctx)
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

func refreshCatalog(ctx context.Context) {
	if _, err := catalog.Update(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "[warning] 节点目录更新失败，使用本地目录:", err)
	}
}
