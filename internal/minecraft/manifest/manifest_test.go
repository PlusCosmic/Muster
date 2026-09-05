package manifest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const good = `{"packs":[{"id":"frontier","name":"Frontier","pack":"https://pack.example/abc/pack.toml",
  "java":{"minMemoryMb":4096,"maxMemoryMb":8192,"args":["-XX:+UseZGC"]},"server":"play.example"}]}`

func TestParseGood(t *testing.T) {
	m, err := Parse([]byte(good))
	if err != nil || len(m.Packs) != 1 || m.Packs[0].Java.MaxMemoryMb != 8192 || m.Packs[0].Server != "play.example" {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestParseNormalisesEmpty(t *testing.T) {
	m, err := Parse([]byte(`{}`))
	if err != nil || m.Packs == nil || len(m.Packs) != 0 {
		t.Fatalf("%+v %v", m, err)
	}
	m, _ = Parse([]byte(`{"packs":[{"id":"a","name":"A","pack":"https://x/p.toml"}]}`))
	if m.Packs[0].Java.Args == nil {
		t.Fatal("args should be [] not nil")
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"bad id":    `{"packs":[{"id":"Cobble mon","name":"x","pack":"https://x/p.toml"}]}`,
		"dup id":    `{"packs":[{"id":"a","name":"x","pack":"https://x/p.toml"},{"id":"a","name":"y","pack":"https://x/q.toml"}]}`,
		"no name":   `{"packs":[{"id":"a","pack":"https://x/p.toml"}]}`,
		"bad url":   `{"packs":[{"id":"a","name":"x","pack":"file:///etc/passwd"}]}`,
		"min > max": `{"packs":[{"id":"a","name":"x","pack":"https://x/p.toml","java":{"minMemoryMb":9,"maxMemoryMb":8}}]}`,
		"not json":  `nope`,
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/m.json" {
			_, _ = w.Write([]byte(good))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	m, err := Fetch(context.Background(), nil, "test", srv.URL+"/m.json")
	if err != nil || len(m.Packs) != 1 {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := Fetch(context.Background(), nil, "test", srv.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404, got %v", err)
	}
}
