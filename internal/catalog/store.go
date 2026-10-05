package catalog

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const configURL = "https://dlcv2.cnspeedtest.cn:8443/TaierAndroid/Config/ConfigMD5.php"
const maxDirectorySize = 8 * 1024 * 1024

type UpdateResult struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Count   int    `json:"count"`
	Updated bool   `json:"updated"`
}
type store struct {
	mu          sync.RWMutex
	updateMu    sync.Mutex
	path        func() (string, error)
	initialized bool
	all         []Server
	encrypted   []byte
	source      string
}

var defaultStore = &store{path: Path}

// Path is shared by the desktop and CLI; files retain the vendor's encrypted format.
func Path() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "globalspeed", "serverlist.json"), nil
}
func (s *store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return
	}
	s.initialized = true
	if path, err := s.path(); err == nil {
		if data, err := readCache(path); err == nil {
			if rows, err := decodeDirectory(data); err == nil {
				s.all = rows
				s.encrypted = data
				s.source = "cache"
				return
			}
		}
	}
	data, err := assets.ReadFile("serverlist_encrypt.json")
	if err != nil {
		panic(err)
	}
	rows, err := decodeDirectory(data)
	if err != nil {
		panic(fmt.Sprintf("内置节点目录损坏: %v", err))
	}
	s.all = rows
	s.encrypted = data
	s.source = "embedded"
}
func readCache(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f, maxDirectorySize)
}
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("节点目录响应过大")
	}
	return data, nil
}
func (s *store) servers() []Server {
	s.load()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Server(nil), s.all...)
}
func (s *store) result(updated bool) UpdateResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, _ := s.path()
	return UpdateResult{Path: path, Source: s.source, Count: len(s.all), Updated: updated}
}
func hash(data []byte) string { sum := md5.Sum(data); return hex.EncodeToString(sum[:]) }

func Update(ctx context.Context) (UpdateResult, error) {
	client := &http.Client{Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext, ResponseHeaderTimeout: 4 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	return defaultStore.update(ctx, client, configURL)
}
func (s *store) update(ctx context.Context, client *http.Client, indexURL string) (UpdateResult, error) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.load()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	q := url.Values{"province": {""}, "city": {""}, "imei": {""}, "appname": {"GlobalSpeed"}, "vercode": {"40408"}, "type": {"globalspeed"}, "ipv6": {"0"}}
	index, err := download(ctx, client, indexURL+"?"+q.Encode(), 1024*1024)
	if err != nil {
		return s.result(false), err
	}
	var entries []struct {
		Name     string `json:"name"`
		Filename string `json:"filename"`
		MD5      string `json:"md5"`
	}
	if json.Unmarshal(index, &entries) != nil {
		return s.result(false), errors.New("节点配置索引无效")
	}
	target, expected := "", ""
	for _, entry := range entries {
		if entry.Name == "serverlist_encrypt_url" {
			target = entry.Filename
			expected = strings.ToLower(entry.MD5)
			break
		}
	}
	digest, err := hex.DecodeString(expected)
	if err != nil || len(digest) != md5.Size || target == "" {
		return s.result(false), errors.New("节点配置索引缺少有效目录信息")
	}
	// The real index may select a different vendor CDN; it cannot redirect to a local address.
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.User != nil || !(u.Hostname() == "cnspeedtest.cn" || strings.HasSuffix(u.Hostname(), ".cnspeedtest.cn")) {
		return s.result(false), errors.New("节点目录下载地址无效")
	}
	s.mu.RLock()
	current := append([]byte(nil), s.encrypted...)
	s.mu.RUnlock()
	data := current
	if hash(current) != expected {
		data, err = download(ctx, client, target, maxDirectorySize)
		if err != nil {
			return s.result(false), err
		}
	}
	if hash(data) != expected {
		return s.result(false), errors.New("节点目录 MD5 校验失败")
	}
	rows, err := decodeDirectory(data)
	if err != nil {
		return s.result(false), err
	}
	path, err := s.path()
	if err != nil {
		return s.result(false), err
	}
	cached, cacheErr := readCache(path)
	updated := cacheErr != nil || hash(cached) != expected
	if updated {
		if err = writeCache(path, data); err != nil {
			return s.result(false), fmt.Errorf("节点目录缓存保存失败: %w", err)
		}
	}
	s.mu.Lock()
	s.all = rows
	s.encrypted = data
	s.source = "cache"
	s.mu.Unlock()
	return s.result(updated), nil
}
func download(ctx context.Context, client *http.Client, target string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, errors.New("节点目录请求无效")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("节点目录更新连接失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("节点目录服务返回 HTTP %d", resp.StatusCode)
	}
	return readLimited(resp.Body, limit)
}
func writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "serverlist-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
