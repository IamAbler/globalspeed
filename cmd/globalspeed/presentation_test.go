package main

import (
	"bytes"
	"globalspeed/internal/catalog"
	"globalspeed/internal/networkinfo"
	"globalspeed/internal/ping"
	"globalspeed/internal/speed"
	"strings"
	"testing"
)

func TestPlainSummaryAndPhaseCompletion(t *testing.T) {
	var out bytes.Buffer
	p := presentation{out: &out, duration: 5, budget: speed.MiB}
	p.header()
	p.server(catalog.Server{ID: "1503", Name: "南京电信", Province: "江苏", City: "南京"}, &networkinfo.Info{IP: "203.0.113.9", Operator: "电信"})
	latency := &ping.Result{Method: "icmp", Status: 0, AverageMS: 29.28, JitterMS: 1.4, Sent: 10, Received: 10}
	p.progress(speed.Progress{Phase: "session", Ping: latency})
	p.progress(speed.Progress{Phase: "download", Mbps: 120, Bytes: 512})
	// The core's last phase event contains the true mean, not its latest instantaneous sample.
	p.progress(speed.Progress{Phase: "download", Mbps: 92, Bytes: speed.MiB})
	p.progress(speed.Progress{Phase: "upload", Mbps: 50, Bytes: 512})
	p.complete(speed.Result{Ping: latency, Download: speed.Transfer{Mbps: 92, Bytes: speed.MiB}, Upload: speed.Transfer{Mbps: 81, Bytes: speed.MiB}, Released: true})
	text := out.String()
	for _, expected := range []string{"GlobalSpeed", "Server: 南京电信", "ISP: 电信", "External IP: 203.0.113.9", "29.28 ms", "jitter: 1.40 ms", "92.00 Mbps", "81.00 Mbps", "Packet Loss:"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
	if strings.ContainsAny(text, "\r\x1b") || strings.Count(text, "Download:") != 1 || strings.Contains(text, "120.00") {
		t.Fatal(text)
	}
}
func TestLiveProgressClearsAndDoesNotInventPacketLoss(t *testing.T) {
	var out bytes.Buffer
	p := presentation{out: &out, live: true, duration: 5, budget: speed.MiB}
	p.progress(speed.Progress{Phase: "download", Mbps: 1, Bytes: speed.MiB / 2, ElapsedMS: 1000})
	if !strings.Contains(out.String(), "50%") {
		t.Fatal(out.String())
	}
	p.clear()
	if p.lineWidth != 0 {
		t.Fatal("line not cleared")
	}
	out.Reset()
	p.complete(speed.Result{Released: true})
	if !strings.Contains(out.String(), "Packet Loss: Unavailable") || !strings.Contains(out.String(), "Idle Latency: Unavailable") {
		t.Fatal(out.String())
	}
	out.Reset()
	p = presentation{out: &out}
	p.complete(speed.Result{Released: true, Ping: &ping.Result{Method: "tcp", Sent: 5, Received: 3, PacketLossPct: 40, Status: 0}})
	if !strings.Contains(out.String(), "TCP Failures:") || strings.Contains(out.String(), "Packet Loss:") {
		t.Fatal(out.String())
	}
}
func TestRemoteTerminalTextIsSanitized(t *testing.T) {
	if strings.ContainsAny(safeText("ISP\n\x1b[2J"), "\n\x1b") {
		t.Fatal("terminal controls preserved")
	}
}
func TestCommandOptionsRejectBeforeNetwork(t *testing.T) {
	for _, args := range [][]string{{"-s", "1503", "--duration", "0"}, {"--format", "csv"}, {"speed", "--progress", "invalid"}, {"speed", "--server", "1503", "--auto"}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
