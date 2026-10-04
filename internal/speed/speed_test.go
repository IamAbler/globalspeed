package speed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenMatchesNativeARM64(t *testing.T) {
	if got := Token("worker-test", "Android", "1791111111", 100); got != "63a2a44c1cdc75882ab4e70952a9a15a" {
		t.Fatalf("token %s", got)
	}
}
func TestDirectProtocolAndBudget(t *testing.T) {
	var uploaded atomic.Int64
	var released atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/speed/dovalid":
			if r.Method == http.MethodPost {
				released.Store(true)
				fmt.Fprint(w, "1")
				return
			}
			q := r.URL.Query()
			if q.Get("token") != Token(q.Get("imei"), q.Get("model"), q.Get("time"), 200) {
				t.Error("wrong native token")
			}
			fmt.Fprint(w, "1-test-key")
		case "/speed/File(1G).dl":
			if r.Header.Get("Accept-Encoding") != "" || r.Header.Get("Cache-Control") != "" || r.Header.Get("Range") != "" {
				t.Error("unexpected download headers")
			}
			if r.URL.Query().Get("key") != "test-key" {
				t.Error("wrong download key")
			}
			w.Write(make([]byte, 256*1024))
		case "/speed/doAnalsLoad.do":
			if r.Header.Get("Key") != "test-key" {
				t.Error("wrong upload key")
			}
			if r.ContentLength <= 0 {
				t.Error("finite multipart length missing")
			}
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, _, err := r.FormFile("upload")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer file.Close()
			n, _ := io.Copy(io.Discard, file)
			uploaded.Add(n)
			fmt.Fprint(w, "OK")
		default:
			t.Error("unexpected measurement target", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	c := NewClient()
	if c.HTTP.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("measurement must bypass external proxies")
	}
	ctx := context.Background()
	key, err := c.session(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	o := Options{DurationSeconds: 1, Connections: 2, MaxMiB: 1}
	for _, phase := range []string{"download", "upload"} {
		r, err := c.transfer(ctx, o, server.URL, key, phase, 64*1024, nil)
		if err != nil {
			t.Fatal(err)
		}
		if r.Bytes != 64*1024 || !r.BudgetReached || r.Mbps <= 0 {
			t.Fatalf("invalid %s: %+v", phase, r)
		}
	}
	if uploaded.Load() != 64*1024 {
		t.Fatal("upload acknowledgement does not match received payload")
	}
	if _, err := c.small(ctx, http.MethodPost, server.URL+"/speed/dovalid?key="+key); err != nil {
		t.Fatal(err)
	}
	if !released.Load() {
		t.Fatal("session was not released")
	}
}
func TestStopCancelsInflightRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := NewClient().transfer(ctx, Options{DurationSeconds: 5, Connections: 1}, server.URL, "test-key", "download", MiB, nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("cancel failed: %v", err)
	}
}

func TestSessionRetriesTransportFailure(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		fmt.Fprint(w, "1-retried-key")
	}))
	defer server.Close()
	key, err := NewClient().session(context.Background(), server.URL)
	if err != nil || key != "retried-key" || attempts.Load() != 3 {
		t.Fatalf("key=%q attempts=%d err=%v", key, attempts.Load(), err)
	}
}

func TestSessionRejectionsAreNotRetried(t *testing.T) {
	for _, reply := range []string{"", "0", "2", "-1"} {
		t.Run("reply="+reply, func(t *testing.T) {
			var attempts atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { attempts.Add(1); fmt.Fprint(w, reply) }))
			defer server.Close()
			_, err := NewClient().session(context.Background(), server.URL)
			if err == nil || attempts.Load() != 1 {
				t.Fatalf("attempts=%d err=%v", attempts.Load(), err)
			}
		})
	}
}
