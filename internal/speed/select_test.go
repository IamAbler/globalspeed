package speed

import (
	"context"
	"fmt"
	"globalspeed/internal/catalog"
	"globalspeed/internal/ping"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginalSelectionOrderAndFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("network") != "5" || q.Get("model") != "Android" || q.Get("province") != "江苏" {
			t.Error("wrong original match parameters")
		}
		fmt.Fprint(w, `[{"hostid":"a","hostname":"A","pname":"江苏","city":"南京","hostip":"218.2.122.246","port":"65499"},{"hostid":"b","hostname":"B","pname":"江苏","city":"南京","hostip":"218.2.122.247","port":"65499"},{"hostid":"c","hostname":"C","pname":"江苏","city":"南京","hostip":"218.2.122.248","port":"65499"}]`)
	}))
	defer server.Close()
	c := NewClient()
	var visited []string
	selected, err := c.match(context.Background(), MatchOptions{Province: "江苏"}, server.URL, func(_ context.Context, s catalog.Server) (ping.Result, error) {
		visited = append(visited, s.ID)
		if s.ID == "a" {
			return ping.Result{Status: 999}, nil
		}
		return ping.Result{Status: 0, AverageMS: 30}, nil
	}, nil)
	if err != nil || selected.ID != "b" || selected.City != "南京" || len(visited) != 2 {
		t.Fatalf("%+v visited=%v err=%v", selected, visited, err)
	}
	selected, err = c.match(context.Background(), MatchOptions{Province: "江苏"}, server.URL, func(context.Context, catalog.Server) (ping.Result, error) { return ping.Result{Status: 999}, nil }, nil)
	if err != nil || selected.ID != "a" {
		t.Fatalf("APK fallback must preserve first candidate: %+v %v", selected, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = c.match(ctx, MatchOptions{}, server.URL, nil, nil); err == nil {
		t.Fatal("cancelled selection succeeded")
	}
}
func TestMatchRejectsPrivateNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"hostid":"a","hostname":"A","hostip":"127.0.0.1","port":"80"}]`)
	}))
	defer server.Close()
	_, err := NewClient().match(context.Background(), MatchOptions{}, server.URL, func(context.Context, catalog.Server) (ping.Result, error) {
		t.Fatal("must reject before probing")
		return ping.Result{}, nil
	}, nil)
	if err == nil {
		t.Fatal("private node accepted")
	}
}
