package mods

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"rimforge/internal/models"
)

const sampleRules = `{
      "timestamp": 1777950016,
      "rules": {
        "Krkr.RocketMan": {
          "loadBottom": { "comment": "always last", "value": true }
        },
        "imranfish.xmlextensions": {
          "loadTop": { "comment": "tier 1", "value": true }
        },
        "3tes.cgtwaa": {
          "loadAfter": { "GT.Sam.GlitterTech": { "name": ["Glitter Tech"] } }
        },
        "ferny.betterarchitect": {
          "loadBefore": { "some.other": { "name": ["Other"] } },
          "incompatibleWith": { "deadmano.rimanoarchitecticons": { "comment": "included" } }
        }
      }
    }`

func TestParsesTheRealUpstreamSchema(t *testing.T) {
	db, err := ParseRules(sampleRules)
	if err != nil {
		t.Fatal(err)
	}
	if db.Timestamp == nil || *db.Timestamp != 1777950016 {
		t.Fatalf("timestamp %v", db.Timestamp)
	}
	if len(db.Rules) != 4 {
		t.Fatalf("got %d rules", len(db.Rules))
	}
	rocketman := db.Rules["krkr.rocketman"]
	if !rocketman.LoadBottom || rocketman.LoadTop {
		t.Fatalf("got %+v", rocketman)
	}
	if !db.Rules["imranfish.xmlextensions"].LoadTop {
		t.Fatal("expected loadTop")
	}
	eq(t, "loadAfter", db.Rules["3tes.cgtwaa"].LoadAfter, []string{"gt.sam.glittertech"})
	arch := db.Rules["ferny.betterarchitect"]
	eq(t, "loadBefore", arch.LoadBefore, []string{"some.other"})
	eq(t, "incompatibleWith", arch.IncompatibleWith, []string{"deadmano.rimanoarchitecticons"})
}

func TestAcceptsABareTopLevelRulesObject(t *testing.T) {
	db, err := ParseRules(`{"a.b": {"loadAfter": {"c.d": {}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "loadAfter", db.Rules["a.b"].LoadAfter, []string{"c.d"})
}

func TestRejectsMalformedJSON(t *testing.T) {
	for _, body := range []string{"{not json", "[1,2,3]"} {
		if _, err := ParseRules(body); err == nil {
			t.Errorf("%q should fail", body)
		}
	}
}

func TestRefreshCachesAndHonoursETag(t *testing.T) {
	t.Setenv("RIMFORGE_DATA_DIR", t.TempDir())
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected a User-Agent")
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(sampleRules))
	}))
	defer srv.Close()
	prevURL, prevClient := rulesURL, httpClient
	rulesURL, httpClient = srv.URL, srv.Client()
	t.Cleanup(func() { rulesURL, httpClient = prevURL, prevClient })

	if st, _ := RulesDbStatus(); st.Cached {
		t.Fatal("nothing should be cached yet")
	}
	st, err := RefreshRulesDb()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Cached || st.RuleCount != 4 || st.FetchedAtMs == nil {
		t.Fatalf("got %+v", st)
	}
	if _, err := os.Stat(rulesPath()); err != nil {
		t.Fatal(err)
	}

	// Second refresh sends the ETag and gets a 304; the cache survives.
	st, err = RefreshRulesDb()
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 || !st.Cached || st.RuleCount != 4 {
		t.Fatalf("hits %d, status %+v", hits, st)
	}
	if db := RulesForSort(); db == nil || len(db.Rules) != 4 {
		t.Fatal("RulesForSort should use the cache")
	}
}

func TestRefreshFailureLeavesNoCache(t *testing.T) {
	t.Setenv("RIMFORGE_DATA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	prevURL, prevClient := rulesURL, httpClient
	rulesURL, httpClient = srv.URL, srv.Client()
	t.Cleanup(func() { rulesURL, httpClient = prevURL, prevClient })

	if _, err := RefreshRulesDb(); err == nil {
		t.Fatal("expected an error")
	}
	if db := RulesForSort(); db != nil {
		t.Fatal("expected no rules")
	}
	if st, _ := RulesDbStatus(); st != (models.RulesDbStatus{}) {
		t.Fatalf("got %+v", st)
	}
}
