package mods

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rimforge/internal/appdir"
	"rimforge/internal/models"
	"rimforge/internal/version"
)

// The RimSort community rules database: fetch, cache, parse.
//
// Source of truth (verified 2026-08-18): the `communityRules.json` file at the
// root of https://github.com/RimSort/Community-Rules-Database, served raw
// from the `main` branch. Actual schema:
//
//	{
//	  "timestamp": 1777950016,
//	  "rules": {
//	    "<packageid>": {
//	      "loadAfter":  { "<other packageid>": { "name": ["Display Name"] } },
//	      "loadBefore": { "<other packageid>": { "name": ["Display Name"] } },
//	      "incompatibleWith": { "<other packageid>": { "name": "…", "comment": "…" } },
//	      "loadTop":    { "comment": "…", "value": true },
//	      "loadBottom": { "comment": "…", "value": true }
//	    }
//	  }
//	}
//
// `loadBottom` (and the undocumented `loadTop`) are `{comment, value: bool}`
// objects, not maps of target package ids. Keys are not reliably lowercase in
// the upstream file, so we lowercase everything on load.

// CommunityRulesURL is the verified raw URL of the community rules database.
const CommunityRulesURL = "https://raw.githubusercontent.com/RimSort/Community-Rules-Database/main/communityRules.json"

const (
	rulesFile = "communityRules.json"
	metaFile  = "rules_meta.json"
)

// ModRule is the community-sourced ordering hints for one mod.
type ModRule struct {
	LoadAfter        []string
	LoadBefore       []string
	IncompatibleWith []string
	LoadTop          bool
	LoadBottom       bool
}

// RulesDb is the parsed database, keyed by lowercase package id.
type RulesDb struct {
	Rules     map[string]ModRule
	Timestamp *int64
}

type rulesMeta struct {
	FetchedAtMs *int64  `json:"fetchedAtMs"`
	Etag        *string `json:"etag"`
}

func rulesPath() string { return filepath.Join(appdir.CacheRoot(), rulesFile) }
func metaPath() string  { return filepath.Join(appdir.CacheRoot(), metaFile) }

// idsOf returns the sorted, lowercased keys of a JSON object value.
func idsOf(value json.RawMessage) []string {
	var m map[string]json.RawMessage
	if value == nil || json.Unmarshal(value, &m) != nil {
		return nil
	}
	var out []string
	for key := range m {
		id := strings.ToLower(strings.TrimSpace(key))
		if id != "" && !containsStr(out, id) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// flagOf reads `{"comment": "...", "value": true}`, also tolerating a bare
// boolean.
func flagOf(value json.RawMessage) bool {
	if value == nil {
		return false
	}
	var b bool
	if json.Unmarshal(value, &b) == nil {
		return b
	}
	var obj struct {
		Value *bool `json:"value"`
	}
	if json.Unmarshal(value, &obj) != nil {
		return false
	}
	return obj.Value == nil || *obj.Value
}

// ParseRules parses the raw JSON into a rules database.
func ParseRules(body string) (*RulesDb, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, fmt.Errorf("communityRules.json parse error: %w", err)
	}

	db := &RulesDb{Rules: map[string]ModRule{}}
	if raw, ok := root["timestamp"]; ok {
		var ts int64
		if json.Unmarshal(raw, &ts) == nil {
			db.Timestamp = &ts
		}
	}
	// Tolerate both `{ "rules": {…} }` and a bare top-level rules object.
	rulesObj := root
	if raw, ok := root["rules"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			rulesObj = nested
		}
	}

	for pid, raw := range rulesObj {
		pid = strings.ToLower(strings.TrimSpace(pid))
		var entry map[string]json.RawMessage
		if pid == "" || pid == "timestamp" || json.Unmarshal(raw, &entry) != nil {
			continue
		}
		db.Rules[pid] = ModRule{
			LoadAfter:        idsOf(entry["loadAfter"]),
			LoadBefore:       idsOf(entry["loadBefore"]),
			IncompatibleWith: idsOf(entry["incompatibleWith"]),
			LoadTop:          flagOf(entry["loadTop"]),
			LoadBottom:       flagOf(entry["loadBottom"]),
		}
	}
	return db, nil
}

// LoadCached loads the cached database, if there is one. Corrupt cache = no cache.
func LoadCached() *RulesDb {
	raw, err := os.ReadFile(rulesPath())
	if err != nil {
		return nil
	}
	db, err := ParseRules(string(raw))
	if err != nil {
		log.Printf("rimforge: discarding corrupt rules cache (%v)", err)
		return nil
	}
	return db
}

func loadMeta() rulesMeta {
	var meta rulesMeta
	if raw, err := os.ReadFile(metaPath()); err == nil {
		_ = json.Unmarshal(raw, &meta)
	}
	return meta
}

func writeMeta(meta rulesMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath(), data, 0o644); err != nil {
		return fmt.Errorf("%s: %w", metaPath(), err)
	}
	return nil
}

func store(body string, etag *string) error {
	if err := appdir.EnsureDir(appdir.CacheRoot()); err != nil {
		return err
	}
	if err := os.WriteFile(rulesPath(), []byte(body), 0o644); err != nil {
		return fmt.Errorf("%s: %w", rulesPath(), err)
	}
	return writeMeta(rulesMeta{FetchedAtMs: models.Int64(time.Now().UnixMilli()), Etag: etag})
}

func touchMeta(etag *string) {
	meta := loadMeta()
	meta.FetchedAtMs = models.Int64(time.Now().UnixMilli())
	if etag != nil {
		meta.Etag = etag
	}
	if err := writeMeta(meta); err != nil {
		log.Printf("rimforge: %v", err)
	}
}

func statusOf(db *RulesDb) models.RulesDbStatus {
	if db == nil {
		return models.RulesDbStatus{}
	}
	meta := loadMeta()
	fetched := meta.FetchedAtMs
	if fetched == nil && db.Timestamp != nil {
		fetched = models.Int64(*db.Timestamp * 1000)
	}
	return models.RulesDbStatus{Cached: true, FetchedAtMs: fetched, RuleCount: len(db.Rules)}
}

// RulesDbStatus is the `get_rules_db_status` command body — cache state only,
// never touches the network.
func RulesDbStatus() (models.RulesDbStatus, error) {
	return statusOf(LoadCached()), nil
}

// httpClient is swapped by tests.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// rulesURL is swapped by tests.
var rulesURL = CommunityRulesURL

// RefreshRulesDb is the `refresh_rules_db` command body — force a re-fetch.
func RefreshRulesDb() (models.RulesDbStatus, error) {
	meta := loadMeta()
	req, err := http.NewRequest(http.MethodGet, rulesURL, nil)
	if err != nil {
		return models.RulesDbStatus{}, fmt.Errorf("http client: %w", err)
	}
	req.Header.Set("User-Agent", "RimForge/"+version.Version)
	if meta.Etag != nil {
		if info, err := os.Stat(rulesPath()); err == nil && info.Mode().IsRegular() {
			req.Header.Set("If-None-Match", *meta.Etag)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return models.RulesDbStatus{}, fmt.Errorf("could not reach the community rules database: %w", err)
	}
	defer resp.Body.Close()

	etag := models.Str(resp.Header.Get("ETag"))
	if resp.StatusCode == http.StatusNotModified {
		touchMeta(etag)
		return statusOf(LoadCached()), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.RulesDbStatus{}, fmt.Errorf("community rules database returned HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.RulesDbStatus{}, fmt.Errorf("could not read the community rules database: %w", err)
	}
	// Validate before we overwrite a working cache.
	db, err := ParseRules(string(body))
	if err != nil {
		return models.RulesDbStatus{}, err
	}
	if err := store(string(body), etag); err != nil {
		return models.RulesDbStatus{}, err
	}
	return statusOf(db), nil
}

// RulesForSort returns the cache, fetching once if it is missing. Never
// fails — nil means "sort with About.xml data only".
func RulesForSort() *RulesDb {
	if db := LoadCached(); db != nil {
		return db
	}
	if _, err := RefreshRulesDb(); err != nil {
		var netErr error = err
		if errors.Is(err, os.ErrNotExist) {
			netErr = err
		}
		log.Printf("rimforge: community rules unavailable (%v)", netErr)
		return nil
	}
	return LoadCached()
}
