package networkinfo

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestIPIPResponse(t *testing.T) {
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ret":"ok","data":{"ip":"203.0.113.9","location":["中国","江苏","南京","","电信"]}}`))}, nil
	})}
	info, err := Lookup(context.Background(), client, Endpoint)
	if err != nil || info.IP != "203.0.113.9" || info.Operator != "电信" || info.Province != "江苏" || info.Location != "中国 · 江苏 · 南京" {
		t.Fatalf("%+v %v", info, err)
	}
}
func TestIPIPInvalidResponses(t *testing.T) {
	for _, body := range []string{`{"ret":"error"}`, `{"ret":"ok","data":{"ip":"invalid"}}`, strings.Repeat("x", 16385)} {
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if _, err := Lookup(context.Background(), client, Endpoint); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
