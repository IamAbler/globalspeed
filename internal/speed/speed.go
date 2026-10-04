package speed

import (
	bytebuffer "bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"globalspeed/internal/catalog"
)

const MiB int64 = 1024 * 1024

type Options struct {
	ServerID        string `json:"serverId"`
	DurationSeconds int    `json:"durationSeconds"`
	Connections     int    `json:"connections"`
	MaxMiB          int    `json:"maxMiB"`
}

func (o Options) Validate() error {
	if o.DurationSeconds < 1 || o.DurationSeconds > 30 {
		return errors.New("每阶段时长必须为 1–30 秒")
	}
	if o.Connections < 1 || o.Connections > 6 {
		return errors.New("并发连接必须为 1–6")
	}
	if o.MaxMiB < 1 || o.MaxMiB > 1024 {
		return errors.New("总流量预算必须为 1–1024 MiB")
	}
	return nil
}

type Progress struct {
	HTTPMedianMS *float64 `json:"httpMedianMs,omitempty"`
	HTTPJitterMS *float64 `json:"httpJitterMs,omitempty"`
	Phase        string   `json:"phase"`
	Mbps         float64  `json:"mbps"`
	Bytes        int64    `json:"bytes"`
	ElapsedMS    int64    `json:"elapsedMs"`
	TCPMedianMS  *float64 `json:"tcpMedianMs,omitempty"`
	TCPJitterMS  *float64 `json:"tcpJitterMs,omitempty"`
}
type Transfer struct {
	Mbps          float64 `json:"mbps"`
	Bytes         int64   `json:"bytes"`
	ElapsedMS     int64   `json:"elapsedMs"`
	BudgetReached bool    `json:"budgetReached"`
}
type Result struct {
	HTTPMedianMS *float64       `json:"httpMedianMs,omitempty"`
	HTTPJitterMS *float64       `json:"httpJitterMs,omitempty"`
	Time         time.Time      `json:"time"`
	Server       catalog.Server `json:"server"`
	Path         string         `json:"path"`
	TCPMedianMS  float64        `json:"tcpMedianMs"`
	TCPJitterMS  float64        `json:"tcpJitterMs"`
	Download     Transfer       `json:"download"`
	Upload       Transfer       `json:"upload"`
	Released     bool           `json:"released"`
}
type Client struct{ HTTP *http.Client }

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Transport: &http.Transport{
		Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		DisableCompression: true, DisableKeepAlives: true, ResponseHeaderTimeout: 5 * time.Second,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func digest(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) }

// Token is reproduced from libGSCore.so 0x77e44 and verified against ARM64 execution.
func Token(identity, model, stamp string, bandwidth int) string {
	return digest(digest("model="+model+"&imei="+identity) + digest(fmt.Sprintf("stime=%s&band=%d&rand=12345555", stamp, bandwidth)))
}
func (c *Client) small(ctx context.Context, method, target string) (string, error) {
	return c.smallWithTimeout(ctx, method, target, 5*time.Second)
}
func (c *Client) smallWithTimeout(ctx context.Context, method, target string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// Avoid including the session URL, token or key in user-facing errors.
		var requestError *url.Error
		if errors.As(err, &requestError) {
			return "", requestError.Err
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("节点返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", errors.New("节点会话响应过大")
	}
	return strings.TrimSpace(string(data)), nil
}
func (c *Client) session(ctx context.Context, base string) (string, error) {
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	identity := "TS" + strings.ToUpper(digest(hex.EncodeToString(seed))[8:24])
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	// Keep the APK's parameter order. Reuse the same identity/token on retry.
	target := base + "/speed/dovalid?key=&flag=true&bandwidth=200&model=Android&imei=" + url.QueryEscape(identity) + "&time=" + stamp + "&app=globalspeed&token=" + Token(identity, "Android", stamp, 200) + "&pkg=com.cnspeedtest.globalspeed"
	var answer string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		answer, err = c.smallWithTimeout(ctx, http.MethodGet, target, 3*time.Second)
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("申请会话失败（APK 异常重试策略，最多 3 次）: %w", err)
	}
	// APK SpeedTestTask.enqueue maps these prefixes to distinct status codes.
	switch {
	case answer == "":
		return "", errors.New("节点会话响应为空（APK 状态 131）")
	case strings.HasPrefix(answer, "0"):
		return "", errors.New("节点拒绝会话：响应以 0 开头（APK 状态 132）")
	case strings.HasPrefix(answer, "2"):
		return "", errors.New("节点拒绝会话：响应以 2 开头（APK 状态 133）")
	case strings.HasPrefix(answer, "-1"):
		return "", errors.New("节点拒绝会话：响应以 -1 开头（APK 状态 134）")
	case !strings.HasPrefix(answer, "1-"):
		return "", errors.New("节点会话响应无法识别")
	}
	key := answer[2:]
	if len(key) < 1 || len(key) > 128 || strings.IndexFunc(key, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.')
	}) >= 0 {
		return "", errors.New("节点会话格式无效")
	}
	return key, nil
}

// tcpConnectSample times DialContext only, excluding local socket shutdown.
func tcpConnectSample(ctx context.Context, address string, dial func(context.Context, string, string) (net.Conn, error), now func() time.Time) (float64, error) {
	start := now()
	conn, err := dial(ctx, "tcp", address)
	elapsed := now().Sub(start)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return float64(elapsed) / float64(time.Millisecond), nil
}
func tcpSamples(ctx context.Context, address string) (float64, float64, error) {
	samples := make([]float64, 0, 5)
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	for i := 0; i < 5; i++ {
		if i > 0 {
			timer := time.NewTimer(150 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return 0, 0, ctx.Err()
			}
		}
		elapsed, err := tcpConnectSample(ctx, address, dialer.DialContext, time.Now)
		if err != nil {
			return 0, 0, fmt.Errorf("TCP 延迟探测失败: %w", err)
		}
		samples = append(samples, elapsed)
	}
	return tcpStatistics(samples)
}
func tcpStatistics(samples []float64) (float64, float64, error) {
	if len(samples) < 2 {
		return 0, 0, errors.New("TCP 延迟样本不足")
	}
	jitter := 0.0
	for i := 1; i < len(samples); i++ {
		d := samples[i] - samples[i-1]
		if d < 0 {
			d = -d
		}
		jitter += d
	}
	jitter /= float64(len(samples) - 1)
	ordered := append([]float64(nil), samples...)
	sort.Float64s(ordered)
	median := ordered[len(ordered)/2]
	if len(ordered)%2 == 0 {
		median = (ordered[len(ordered)/2-1] + median) / 2
	}
	return median, jitter, nil
}

// HTTP latency requires a response, so a locally accepted TCP connection
// alone cannot produce a successful sample. It includes server processing.
func (c *Client) httpSamples(ctx context.Context, target string) (float64, float64, error) {
	samples := make([]float64, 0, 5)
	for i := 0; i < 5; i++ {
		if i > 0 {
			timer := time.NewTimer(150 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return 0, 0, ctx.Err()
			}
		}
		sampleCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		var wrote, first time.Time
		trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wrote = time.Now()
			}
		}, GotFirstResponseByte: func() { first = time.Now() }}
		req, err := http.NewRequestWithContext(httptrace.WithClientTrace(sampleCtx, trace), http.MethodGet, target, nil)
		if err != nil {
			cancel()
			return 0, 0, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.60 Safari/537.36")
		req.Header.Set("Accept", "*/*")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			cancel()
			return 0, 0, errors.New("HTTP 延迟探测未收到有效响应")
		}
		// Read only one payload byte; the large download is stopped immediately.
		_, readErr := io.CopyN(io.Discard, resp.Body, 1)
		resp.Body.Close()
		cancel()
		if resp.StatusCode != http.StatusOK || readErr != nil || wrote.IsZero() || first.IsZero() || first.Before(wrote) {
			return 0, 0, fmt.Errorf("HTTP 延迟探测响应无效（HTTP %d）", resp.StatusCode)
		}
		samples = append(samples, float64(first.Sub(wrote))/float64(time.Millisecond))
	}
	return tcpStatistics(samples)
}
func (c *Client) Run(ctx context.Context, o Options, progress func(Progress)) (result Result, err error) {
	if err = o.Validate(); err != nil {
		return
	}
	result.Time = time.Now()
	result.Path = "device-to-carrier"
	result.Server, err = catalog.Find(o.ServerID)
	if err != nil {
		return
	}
	address, addressErr := result.Server.Address()
	if addressErr != nil {
		err = addressErr
		return
	}
	base := "http://" + address
	if progress != nil {
		progress(Progress{Phase: "latency"})
	}
	result.TCPMedianMS, result.TCPJitterMS, err = tcpSamples(ctx, address)
	if err != nil {
		return
	}
	if progress != nil {
		progress(Progress{Phase: "session", TCPMedianMS: &result.TCPMedianMS, TCPJitterMS: &result.TCPJitterMS})
	}
	key, sessionErr := c.session(ctx, base)
	if sessionErr != nil {
		err = fmt.Errorf("%s (%s) 会话阶段: %w", result.Server.Name, address, sessionErr)
		return
	}
	defer func() {
		// Release with a fresh context even when the user stopped sampling.
		_, releaseErr := c.small(context.Background(), http.MethodPost, base+"/speed/dovalid?key="+url.QueryEscape(key))
		result.Released = releaseErr == nil
		if releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("会话释放未确认: %w", releaseErr))
		}
	}()
	if progress != nil {
		progress(Progress{Phase: "latency"})
	}
	httpMedian, httpJitter, latencyErr := c.httpSamples(ctx, base+"/speed/File(1G).dl?r="+strconv.FormatInt(time.Now().Unix(), 10)+"&key="+url.QueryEscape(key))
	if latencyErr != nil {
		err = fmt.Errorf("HTTP 延迟阶段: %w", latencyErr)
		return
	}
	result.HTTPMedianMS = &httpMedian
	result.HTTPJitterMS = &httpJitter
	if progress != nil {
		progress(Progress{Phase: "latency", HTTPMedianMS: &httpMedian, HTTPJitterMS: &httpJitter})
	}
	budget := int64(o.MaxMiB) * MiB / 2
	result.Download, err = c.transfer(ctx, o, base, key, "download", budget, progress)
	if err != nil {
		return
	}
	result.Upload, err = c.transfer(ctx, o, base, key, "upload", budget, progress)
	return
}
func (c *Client) transfer(parent context.Context, o Options, base, key, phase string, budget int64, progress func(Progress)) (Transfer, error) {
	// Reuse a random payload; generation is outside the measurement interval.
	payload := make([]byte, 256*1024)
	if phase == "upload" {
		if _, err := rand.Read(payload); err != nil {
			return Transfer{}, err
		}
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, time.Duration(o.DurationSeconds)*time.Second)
	defer cancel()
	var bytes, reserved atomic.Int64
	var failure error
	var mu sync.Mutex
	fail := func(err error) {
		if ctx.Err() != nil {
			return
		}
		mu.Lock()
		if failure == nil {
			failure = err
		}
		mu.Unlock()
		cancel()
	}
	tickerDone := make(chan struct{})
	var tickerWG sync.WaitGroup
	if progress != nil {
		progress(Progress{Phase: phase})
		tickerWG.Add(1)
		go func() {
			defer tickerWG.Done()
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			var previous int64
			last := started
			for {
				select {
				case now := <-ticker.C:
					n := bytes.Load()
					progress(Progress{Phase: phase, Mbps: float64(n-previous) * 8 / now.Sub(last).Seconds() / 1e6, Bytes: n, ElapsedMS: now.Sub(started).Milliseconds()})
					previous = n
					last = now
				case <-tickerDone:
					return
				}
			}
		}()
	}
	var wg sync.WaitGroup
	for i := 0; i < o.Connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				block := MiB
				if phase == "upload" {
					block = int64(len(payload))
				}
				var count int64
				for {
					old := reserved.Load()
					if old >= budget {
						return
					}
					count = min(block, budget-old)
					if reserved.CompareAndSwap(old, old+count) {
						break
					}
				}
				target := base + "/speed/File(1G).dl?r=" + strconv.FormatInt(time.Now().Unix(), 10) + "&key=" + url.QueryEscape(key)
				method := http.MethodGet
				var body io.Reader
				var contentLength int64
				if phase == "upload" {
					method = http.MethodPost
					target = base + "/speed/doAnalsLoad.do"
					prefix := "--00content0boundary00\r\nContent-Disposition: form-data; name=\"upload\";filename=\"SPEED_GO\"\r\n\r\n"
					suffix := "\r\n--00content0boundary00--\r\n"
					body = io.MultiReader(strings.NewReader(prefix), bytebuffer.NewReader(payload[:count]), strings.NewReader(suffix))
					contentLength = int64(len(prefix)+len(suffix)) + count
				}
				req, err := http.NewRequestWithContext(ctx, method, target, body)
				if err != nil {
					fail(err)
					return
				}
				if phase == "download" {
					req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.60 Safari/537.36")
					req.Header.Set("Accept", "*/*")
				} else {
					req.Header.Set("Cache-Control", "no-cache")
					req.Header.Set("Charset", "UTF-8")
					req.Header.Set("Key", key)
					req.Header.Set("Content-Type", "multipart/form-data;boundary=00content0boundary00")
					req.Header.Set("User-Agent", "Dalvik/1.6.0 (Linux; U; Android 4.2.2; GT-I9505 Build/JDQ39)")
					req.ContentLength = contentLength
				}
				resp, err := c.HTTP.Do(req)
				if err != nil {
					fail(err)
					return
				}
				if resp.StatusCode != 200 {
					resp.Body.Close()
					fail(fmt.Errorf("%s 返回 HTTP %d", phase, resp.StatusCode))
					return
				}
				if resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
					resp.Body.Close()
					fail(errors.New("节点返回压缩响应，无法按传输字节测速"))
					return
				}
				if phase == "download" {
					buf := make([]byte, 64*1024)
					remaining := count
					for remaining > 0 {
						n, e := resp.Body.Read(buf[:min(int64(len(buf)), remaining)])
						bytes.Add(int64(n))
						remaining -= int64(n)
						if e != nil {
							if remaining > 0 {
								fail(e)
							}
							break
						}
					}
					resp.Body.Close()
				} else {
					_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 4097))
					resp.Body.Close()
					if err != nil {
						fail(err)
						return
					}
					bytes.Add(count)
				}
			}
		}()
	}
	wg.Wait()
	close(tickerDone)
	tickerWG.Wait()
	elapsed := time.Since(started)
	n := bytes.Load()
	result := Transfer{Mbps: float64(n) * 8 / elapsed.Seconds() / 1e6, Bytes: n, ElapsedMS: elapsed.Milliseconds(), BudgetReached: n >= budget}
	if parent.Err() != nil {
		return result, parent.Err()
	}
	if failure != nil {
		return result, fmt.Errorf("%s 阶段: %w", phase, failure)
	}
	if n == 0 {
		return result, errors.New("采样期内没有完成有效传输")
	}
	if progress != nil {
		progress(Progress{Phase: phase, Mbps: result.Mbps, Bytes: n, ElapsedMS: result.ElapsedMS})
	}
	return result, nil
}
