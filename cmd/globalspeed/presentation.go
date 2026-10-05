package main

import (
	"fmt"
	"io"
	"strings"

	"globalspeed/internal/catalog"
	"globalspeed/internal/networkinfo"
	"globalspeed/internal/ping"
	"globalspeed/internal/speed"
)

type presentation struct {
	out             io.Writer
	live            bool
	duration        int
	budget          int64
	lineWidth       int
	phase           string
	last            speed.Progress
	latencyPrinted  bool
	downloadPrinted bool
}

func (p *presentation) clear() {
	if p.lineWidth > 0 {
		fmt.Fprintf(p.out, "\r%s\r", strings.Repeat(" ", p.lineWidth))
		p.lineWidth = 0
	}
}
func (p *presentation) status(text string) {
	if !p.live {
		return
	}
	p.clear()
	fmt.Fprint(p.out, text)
	p.lineWidth = len(text)
}
func (p *presentation) header() { fmt.Fprint(p.out, "\n   GlobalSpeed\n\n") }
func (p *presentation) server(s catalog.Server, n *networkinfo.Info) {
	p.clear()
	place := strings.Join(nonempty(s.Province, s.City), " · ")
	fmt.Fprintf(p.out, "%13s %s", "Server:", safeText(s.Name))
	if place != "" {
		fmt.Fprintf(p.out, " - %s", safeText(place))
	}
	fmt.Fprintf(p.out, " (id: %s)\n", safeText(s.ID))
	isp, ip := "Unavailable", "Unavailable"
	if n != nil {
		ip = n.IP
		if n.Operator != "" {
			isp = safeText(n.Operator)
		}
	}
	fmt.Fprintf(p.out, "%13s %s\n%13s %s\n", "ISP:", isp, "External IP:", ip)
}

// Avoid control sequences from remote metadata changing the terminal display.
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, s)
}
func nonempty(fields ...string) []string {
	var result []string
	for _, s := range fields {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}
func (p *presentation) latency(r *ping.Result) {
	p.clear()
	p.latencyPrinted = true
	if r == nil || r.Status != 0 {
		fmt.Fprintf(p.out, "%13s Unavailable\n", "Idle Latency:")
		return
	}
	fmt.Fprintf(p.out, "%13s %8.2f ms   (jitter: %.2f ms, %s)\n", "Idle Latency:", r.AverageMS, r.JitterMS, strings.ToUpper(r.Method))
}
func (p *presentation) transfer(label string, t speed.Transfer) {
	p.clear()
	fmt.Fprintf(p.out, "%13s %8.2f Mbps (data used: %.2f MiB)\n", label+":", t.Mbps, float64(t.Bytes)/float64(speed.MiB))
}
func (p *presentation) progress(v speed.Progress) {
	if v.Phase != p.phase {
		p.clear()
		if p.phase == "download" && v.Phase == "upload" {
			p.transfer("Download", speed.Transfer{Mbps: p.last.Mbps, Bytes: p.last.Bytes})
			p.downloadPrinted = true
		}
		p.phase = v.Phase
	}
	if v.Phase == "session" && v.Ping != nil && !p.latencyPrinted {
		p.latency(v.Ping)
	}
	p.last = v
	if v.Phase == "download" || v.Phase == "upload" {
		percent := 0.0
		if p.duration > 0 {
			percent = float64(v.ElapsedMS) / float64(p.duration*1000)
		}
		if p.budget > 0 {
			percent = max(percent, float64(v.Bytes)/float64(p.budget))
		}
		percent = min(1, max(0, percent))
		filled := int(percent * 16)
		label := "Download:"
		if v.Phase == "upload" {
			label = "Upload:"
		}
		p.status(fmt.Sprintf("%13s %8.2f Mbps [%s%s] %3.0f%%", label, v.Mbps, strings.Repeat("=", filled), strings.Repeat(".", 16-filled), percent*100))
	} else if v.Phase == "latency" {
		p.status(fmt.Sprintf("%13s Measuring...", "Idle Latency:"))
	} else if v.Phase == "session" {
		p.status(fmt.Sprintf("%13s Connecting...", "Session:"))
	}
}
func (p *presentation) complete(r speed.Result) {
	p.clear()
	if !p.latencyPrinted {
		p.latency(r.Ping)
	}
	if !p.downloadPrinted {
		p.transfer("Download", r.Download)
	}
	p.transfer("Upload", r.Upload)
	label := "Packet Loss:"
	if r.Ping != nil && r.Ping.Method == "tcp" {
		label = "TCP Failures:"
	}
	if r.Ping == nil || r.Ping.Sent == 0 {
		fmt.Fprintf(p.out, "%13s Unavailable\n", label)
	} else {
		fmt.Fprintf(p.out, "%13s %8.1f%%\n", label, r.Ping.PacketLossPct)
	}
	if !r.Released {
		fmt.Fprintf(p.out, "%13s Release not confirmed\n", "Session:")
	}
	fmt.Fprintln(p.out)
}
