package virtualcluster

import (
	"strings"
	"testing"
)

const validSpec = `
apiVersion: forgelab/v1
kind: Cluster
name: prod
nodes:
  - name: web
    count: 100
    profile: large
  - name: db
    count: 3
    resources:
      cpuCores: 32
      memory: 128Gi
      disk: 4Ti
`

func TestParseAndTotal(t *testing.T) {
	c, err := Parse([]byte(validSpec))
	if err != nil {
		t.Fatal(err)
	}
	if c.Nodes() != 103 {
		t.Fatalf("nodes = %d", c.Nodes())
	}
	tot := c.Total()
	if tot.CPUCores != 100*16+3*32 {
		t.Errorf("cpu = %v", tot.CPUCores)
	}
	if tot.MemoryBytes != 100*(64<<30)+3*(128<<30) {
		t.Errorf("memory = %v", tot.MemoryBytes)
	}
	if tot.DiskBytes != 100*(2<<40)+3*(4<<40) {
		t.Errorf("disk = %v", tot.DiskBytes)
	}
}

func TestParseErrors(t *testing.T) {
	head := "apiVersion: forgelab/v1\nkind: Cluster\nname: x\n"
	cases := map[string]string{
		"wrong version":  strings.Replace(validSpec, "forgelab/v1", "v2", 1),
		"wrong kind":     strings.Replace(validSpec, "kind: Cluster", "kind: Other", 1),
		"no nodes":       head,
		"zero count":     head + "nodes:\n  - name: a\n    count: 0\n    profile: small\n",
		"unknown prof":   head + "nodes:\n  - name: a\n    count: 1\n    profile: huge\n",
		"both":           head + "nodes:\n  - name: a\n    count: 1\n    profile: small\n    resources: {cpuCores: 1, memory: 1Gi, disk: 1Gi}\n",
		"neither":        head + "nodes:\n  - name: a\n    count: 1\n",
		"duplicate":      head + "nodes:\n  - {name: a, count: 1, profile: small}\n  - {name: a, count: 1, profile: small}\n",
		"unknown field":  head + "bogus: 1\nnodes:\n  - {name: a, count: 1, profile: small}\n",
		"bad quantity":   head + "nodes:\n  - name: a\n    count: 1\n    resources: {cpuCores: 1, memory: lots, disk: 1Gi}\n",
		"incomplete res": head + "nodes:\n  - name: a\n    count: 1\n    resources: {cpuCores: 1}\n",
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseBytes(t *testing.T) {
	cases := map[string]Bytes{"1024": 1024, "1Ki": 1024, "2Gi": 2 << 30, "1T": 1e12, "1.5Ki": 1536}
	for in, want := range cases {
		got, err := ParseBytes(in)
		if err != nil || got != want {
			t.Errorf("%q: got %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-1Gi"} {
		if _, err := ParseBytes(in); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}

func TestDatabase(t *testing.T) {
	doc := validSpec + "database:\n  maxConnections: 2000\n  maxQps: 50000\n  storage: 10Ti\n"
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Capacity(); got.Database.MaxConnections != 2000 || got.Database.StorageBytes != 10<<40 || got.Compute != c.Total() {
		t.Fatalf("capacity = %+v", got)
	}
	if _, err := Parse([]byte(validSpec + "database:\n  maxConnections: 0\n  maxQps: 1\n  storage: 1Gi\n")); err == nil {
		t.Fatal("expected error for zero maxConnections")
	}
}
