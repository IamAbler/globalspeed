package speed

import (
	"context"
	"errors"
	"fmt"
	"globalspeed/internal/catalog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPKSessionStatus(t *testing.T) {
	for _, tc := range []struct {
		reply   string
		code    int
		message string
	}{
		{"", 131, "测速服务器无响应"}, {"0", 132, "测速服务器繁忙,请稍后重试"},
		{"2", 133, "测速服务器忙，请稍后再试"}, {"-1", 134, "请求排队参数错误"},
	} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.reply) }))
			defer server.Close()
			_, err := NewClient().session(context.Background(), server.URL)
			status := PublicError(err)
			if status.Code != tc.code || status.Message != tc.message {
				t.Fatalf("status=%+v", status)
			}
		})
	}
}
func TestPublicErrorPreservesCauseAndHidesDetails(t *testing.T) {
	cause := errors.New("request includes private session credentials")
	err := Failure(130, cause)
	if err.Error() != "测速服务器连接失败" || !errors.Is(err, cause) {
		t.Fatal(err)
	}
	if PublicError(fmt.Errorf("wrapped: %w", context.Canceled)).Code != 990 {
		t.Fatal("cancel mapping")
	}
}

func TestSessionHTTPFailureIsNotRetried(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { attempts++; w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	_, err := NewClient().session(context.Background(), server.URL)
	if PublicError(err).Code != 131 || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRunSelectedUsesMatchedServerOutsideCatalog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	selected := catalog.Server{ID: "new-match-only", IP: "203.0.113.8", Port: "65499", Name: "Matched server"}
	result, err := NewClient().RunSelected(ctx, Options{Auto: true, DurationSeconds: 1, Connections: 1, MaxMiB: 1}, selected, nil)
	if result.Server.ID != selected.ID || PublicError(err).Code != 990 {
		t.Fatalf("server=%+v err=%v", result.Server, err)
	}
}
