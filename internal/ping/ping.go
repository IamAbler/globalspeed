package ping

import (
	"context"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Result struct {
	Method         string  `json:"method"`
	AverageMS      float64 `json:"averageMs"`
	JitterMS       float64 `json:"jitterMs"`
	Sent           int     `json:"sent"`
	Received       int     `json:"received"`
	PacketLossPct  float64 `json:"packetLossPct"`
	APKPackageLost *int    `json:"apkPackageLost,omitempty"`
	Status         int     `json:"status"`
}

var replyTime = regexp.MustCompile(`(?i)(?:time|时间|時間)\s*([=<])\s*([0-9]+(?:[.,][0-9]+)?)\s*(?:ms|毫秒)`)
var summaryAverage = regexp.MustCompile(`(?:rtt|round-trip)\s+min/avg/max/(?:mdev|stddev)\s*=\s*[0-9.]+/([0-9.]+)/`)

func Parse(output string, count int) Result {
	r := Result{Method: "icmp", Sent: count, Status: 999}
	var samples []float64
	for _, line := range strings.Split(output, "\n") {
		// Windows replies have TTL=; Unix replies contain "bytes from".
		if !strings.Contains(strings.ToLower(line), "ttl=") && !strings.Contains(line, "bytes from") {
			continue
		}
		m := replyTime.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", "."), 64)
		if err != nil {
			continue
		}
		// Windows exposes only an upper bound for sub-millisecond replies.
		if m[1] == "<" {
			n = 0
		}
		samples = append(samples, n)
	}
	r.Received = len(samples)
	if count > 0 {
		r.PacketLossPct = float64(count-r.Received) * 100 / float64(count)
	}
	var sum, diff float64
	for i, n := range samples {
		sum += n
		if i > 0 && samples[i-1] > .01 {
			diff += abs(n - samples[i-1])
		}
	}
	if r.Received > 0 {
		r.AverageMS = sum / float64(r.Received)
	}
	if m := summaryAverage.FindStringSubmatch(output); m != nil {
		r.AverageMS, _ = strconv.ParseFloat(m[1], 64)
	}
	if r.Received > 1 {
		r.JitterMS = diff / float64(r.Received-1)
	}
	if r.AverageMS >= .01 {
		r.Status = 0
	}
	return r
}
func abs(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func Measure(ctx context.Context, ip, port string) (Result, error) {
	count := 5
	quick := false
	if runtime.GOOS != "windows" {
		helpCtx, cancel := context.WithTimeout(ctx, time.Second)
		cmd := exec.CommandContext(helpCtx, "ping", "-h")
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		help, _ := cmd.CombinedOutput()
		cancel()
		quick = strings.Contains(string(help), "interval")
		if quick {
			count = 10
		}
	}
	args := []string{"-s", "64", "-c", strconv.Itoa(count), "-W", "3"}
	switch runtime.GOOS {
	case "windows":
		args = []string{"-n", strconv.Itoa(count), "-l", "64", "-w", "3000"}
	case "darwin":
		args = []string{"-s", "64", "-c", strconv.Itoa(count), "-W", "3000"}
	}
	if quick {
		args = append([]string{"-i", "0.2"}, args...)
	}
	args = append(args, ip)
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(count)*4*time.Second)
	cmd := exec.CommandContext(pingCtx, "ping", args...)
	hideConsole(cmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, _ := cmd.CombinedOutput()
	cancel()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if runtime.GOOS == "windows" && !utf8.Valid(output) {
		if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(output); err == nil {
			output = decoded
		}
	}
	r := Parse(string(output), count)
	if r.AverageMS < .1 {
		return TCP(ctx, net.JoinHostPort(ip, port), 5, 3*time.Second)
	}
	return r, nil
}

// TCP reproduces PingTask's millisecond quantisation and integer divisions,
// including its legacy packageLost field (success percentage, not true loss).
func TCP(ctx context.Context, address string, count int, timeout time.Duration) (Result, error) {
	r := Result{Method: "tcp", Sent: count, Status: 999}
	var samples []int64
	for i := 0; i < count; i++ {
		start := time.Now().UnixMilli()
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
		elapsed := time.Now().UnixMilli() - start
		if conn != nil {
			conn.Close()
		}
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		if err == nil && elapsed < timeout.Milliseconds() {
			samples = append(samples, elapsed)
		}
		if err := wait(ctx, 50*time.Millisecond); err != nil {
			return r, err
		}
	}
	return TCPStatistics(samples, count), nil
}
func TCPStatistics(samples []int64, count int) Result {
	if count < 1 {
		return Result{Method: "tcp", Status: 999}
	}
	r := Result{Method: "tcp", Sent: count, Received: len(samples), Status: 999}
	legacyLost := 0
	r.APKPackageLost = &legacyLost
	r.PacketLossPct = float64(count-r.Received) * 100 / float64(count)
	var sum, diff, last int64
	for _, n := range samples {
		sum += n
		if last != 0 {
			d := n - last
			if d < 0 {
				d = -d
			}
			diff += d
		}
		last = n
	}
	if r.Received > 1 {
		r.AverageMS = float64(sum / int64(r.Received))
		r.JitterMS = float64(diff / int64(r.Received))
		legacyLost := (r.Received / count) * 100
		r.APKPackageLost = &legacyLost
	}
	if r.AverageMS >= .01 {
		r.Status = 0
	}
	return r
}
func (r Result) Description() string {
	if r.Status != 0 {
		return strings.ToUpper(r.Method) + " Ping 未获得有效结果"
	}
	return fmt.Sprintf("%s 平均 %.2f ms · 抖动 %.2f ms", strings.ToUpper(r.Method), r.AverageMS, r.JitterMS)
}
