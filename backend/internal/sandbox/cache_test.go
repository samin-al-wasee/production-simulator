package sandbox

import (
	"math"
	"reflect"
	"testing"
)

func newGameV9(t *testing.T) *Game {
	t.Helper()
	r := RulesetV9()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

// cached builds traffic → app → cache → database and returns the app,
// cache, and database.
func cached(t *testing.T, g *Game, rps float64) (string, string, string) {
	t.Helper()
	app, cache, db := place(t, g, KindApp), place(t, g, KindCache), place(t, g, KindDBPrimary)
	connect(t, g, app, cache)
	connect(t, g, cache, db)
	connect(t, g, app, db)
	c := constant(client(), rps)
	c.Endpoints = []Weight{{"GET /products", 1}}
	traffic(t, g, app, c)
	return app, cache, db
}

func cacheStats(t *testing.T, g *Game, id string) CacheStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Cache == nil {
		t.Fatalf("%s has no cache stats", id)
	}
	return *s.Cache
}

func TestAColdCacheWarmsUpAndThenHits(t *testing.T) {
	g := newGameV9(t)
	_, cache, db := cached(t, g, 20)
	g.Step()
	cold := cacheStats(t, g, cache)
	if cold.HitRatio != 0 || !near(stats(t, g.Last, db).Offered, 20, 1e-6) {
		t.Fatalf("a new cache is empty: every read misses to the database: %+v", cold)
	}
	for range 3 {
		g.Step()
	}
	st := cacheStats(t, g, cache)
	hot := st.Keys * g.Rules.CacheRuntime.HotKeyShare
	want := 1 - math.Exp(-300*20/hot) // everything fits; TTL 300 s on the hot keys
	if st.Warmth != 1 || !near(st.HitRatio, want, 1e-9) || st.Fits != 1 {
		t.Fatalf("a warm cache hits as often as its hot keys stay fresh: %+v, want %v", st, want)
	}
	if db := stats(t, g.Last, db).Offered; !near(db, 20*(1-want), 1e-6) {
		t.Fatalf("only misses reach the database: %v", db)
	}
}

func TestEvictionPoliciesAndDataThatOutgrowsMemory(t *testing.T) {
	g := newGameV9(t)
	_, cache, _ := cached(t, g, 2000)
	for range 3 {
		g.Step()
	}
	g.Users = 2_000_000
	hit := map[string]float64{}
	for _, ev := range []string{EvictLRU, EvictLFU, EvictNone} {
		c := *g.Rules.Cache
		c.Eviction = ev
		must(t, g, Command{Type: CmdConfigure, Node: cache, Cache: &c})
		hit[ev] = cacheStats(t, g, cache).HitRatio
	}
	fits := cacheStats(t, g, cache).Fits
	if !(fits < 0.1) || !(hit[EvictLFU] > hit[EvictLRU] && hit[EvictLRU] > hit[EvictNone]) {
		t.Fatalf("with %v of the working set in memory, LFU > LRU > none: %v", fits, hit)
	}
	if st := cacheStats(t, g, cache); !(st.Evictions > 0) {
		t.Fatalf("a full cache evicts: %+v", st)
	}
}

func TestAShortTTLExpiresHotKeys(t *testing.T) {
	g := newGameV9(t)
	_, cache, _ := cached(t, g, 20)
	for range 3 {
		g.Step()
	}
	long := cacheStats(t, g, cache).HitRatio
	c := *g.Rules.Cache
	c.TTLSeconds = 1
	must(t, g, Command{Type: CmdConfigure, Node: cache, Cache: &c})
	if short := cacheStats(t, g, cache).HitRatio; !(short < long/10) {
		t.Fatalf("hot keys expire before they are read again: %v vs %v", short, long)
	}
}

func TestARestartedCacheIsColdAgain(t *testing.T) {
	g := newGameV9(t)
	_, cache, db := cached(t, g, 20)
	for range 3 {
		g.Step()
	}
	g.Events = []*Event{{Effect: EffectCrash, Start: g.Tick, End: g.Tick + 1, Hits: []Hit{{Node: cache, Replicas: 1}}}}
	g.Step()
	g.Step()
	if st := cacheStats(t, g, cache); st.HitRatio != 0 || !near(stats(t, g.Last, db).Offered, 20, 1e-6) {
		t.Fatalf("after a restart every read misses again: %+v", st)
	}
}

func TestLargeValuesFillTheNetwork(t *testing.T) {
	g := newGameV9(t)
	_, cache, _ := cached(t, g, 20)
	c := *g.Rules.Cache
	c.ValueKB = 1000
	must(t, g, Command{Type: CmdConfigure, Node: cache, Cache: &c})
	if st := cacheStats(t, g, cache); st.Bottleneck != "network" || !near(st.Capacity, 100/(1000*8.0/1000), 1e-9) {
		t.Fatalf("1 MB values cap a small cache at 100 Mbps ÷ 8 Mb: %+v", st)
	}
}

func TestCacheConfigValidatesAndReplays(t *testing.T) {
	r := RulesetV9()
	if p := r.ValidateCache(*r.Cache); len(p) != 0 {
		t.Fatalf("the default is valid: %v", p)
	}
	if p := r.ValidateCache(CacheConfig{Eviction: "random"}); len(p) != 4 {
		t.Fatalf("every problem is listed: %v", p)
	}
	g := New(RulesetV9(), 2)
	app := appWithDeps(t, g)
	traffic(t, g, app, client())
	for range 400 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || !reflect.DeepEqual(again.Nodes, g.Nodes) {
		t.Fatalf("a v9 game must replay exactly: %v", err)
	}
}

func TestFreeBuildUnlocksEverythingAndReplays(t *testing.T) {
	g := New(RulesetV9(), 1)
	if _, err := g.Apply(Command{Type: CmdPlace, Kind: KindCache}); err == nil {
		t.Fatal("a cache is locked until the startup tier")
	}
	g = New(RulesetV9(), 1)
	g.FreeBuild = true
	id := must(t, g, Command{Type: CmdPlace, Kind: KindCache})
	again, err := Replay(g.Save())
	if err != nil || again.Node(id) == nil || !again.FreeBuild {
		t.Fatalf("a free-build save replays as free build: %v", err)
	}
}
