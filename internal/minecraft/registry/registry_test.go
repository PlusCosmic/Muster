package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeCode(t *testing.T) {
	good := map[string]string{
		"amber-otter-42":                              "amber-otter-42",
		"  Amber Otter 42 ":                           "amber-otter-42",
		"muster://add/amber-otter-42":                 "amber-otter-42",
		"https://musterlauncher.com/p/plum-weasel-23": "plum-weasel-23",
		"7K3P-QW9X":                                   "7k3p-qw9x",
	}
	for in, want := range good {
		got, err := NormalizeCode(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "   ", "amber", "https://x/", "amber_otter"} {
		if _, err := NormalizeCode(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/packs/plum-weasel-23":
			_, _ = w.Write([]byte(`{"code":"plum-weasel-23","pack":{"id":"frontier","name":"Frontier","pack":"https://packs.example.com/pack.toml","server":"play.example.com"},"createdAt":"2026-09-05T20:23:49Z","updatedAt":"2026-09-05T20:23:49Z","resolved":{"version":"1.0.0","minecraft":"1.21.1","loader":"neoforge","loaderVersion":"21.1.248","checkedAt":"2026-09-05T20:23:49Z"}}`))
		case "/v1/packs/bad-entry-1":
			_, _ = w.Write([]byte(`{"code":"bad-entry-1","pack":{"id":"x","name":"X","pack":"ftp://nope"}}`))
		case "/v1/packs/rate-limited-9":
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no pack registered with that code"}}`))
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL + "/"}
	reg, err := c.Resolve(context.Background(), "plum-weasel-23")
	if err != nil || reg.Pack.ID != "frontier" || reg.Resolved == nil || reg.Resolved.Loader != "neoforge" || reg.Pack.Recommended.Args == nil {
		t.Fatalf("%+v %v", reg, err)
	}
	if _, err := c.Resolve(context.Background(), "nobody-home-1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := c.Resolve(context.Background(), "bad-entry-1"); err == nil {
		t.Fatal("unusable entry should be refused")
	}
	var re *Error
	if _, err := c.Resolve(context.Background(), "rate-limited-9"); err == nil || !asErr(err, &re) || re.Code != "rate_limited" {
		t.Fatalf("%v", err)
	}
}

func asErr(err error, target **Error) bool {
	e, ok := err.(*Error)
	if ok {
		*target = e
	}
	return ok
}
