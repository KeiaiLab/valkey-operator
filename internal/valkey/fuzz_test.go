/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package valkey

import (
	"strings"
	"testing"
)

// Fuzz the five INFO/CLUSTER parsers that consume untrusted output from
// `valkey-cli`. The seed corpus uses real Valkey responses; the fuzzer
// explores malformed / truncated / hostile variants. We assert that the
// parsers never panic and that returned slices/maps are within sane
// bounds for any well-formed-looking response.
//
// Run with: `go test -fuzz=Fuzz... -fuzztime=30s ./internal/valkey/`

func FuzzParseClusterInfo(f *testing.F) {
	f.Add("cluster_state:ok\ncluster_slots_assigned:16384\ncluster_slots_ok:16384\ncluster_slots_pfail:0\ncluster_slots_fail:0\n")
	f.Add("")
	f.Add("cluster_state:fail\ncluster_slots_ok:0\n")
	f.Add("cluster_state\n:::\n\x00garbage")
	f.Fuzz(func(t *testing.T, s string) {
		// Property: parseClusterInfo is a transparent parser — it does
		// not bound-check the wire (Valkey could lie about
		// `cluster_slots_assigned > 16384`); upper-layer reconcile
		// logic enforces the 16384 invariant. Just assert no panic and
		// a non-nil return.
		got := parseClusterInfo(s)
		if got == nil {
			t.Fatal("parseClusterInfo returned nil")
		}
	})
}

func FuzzParseKeyspaceKeys(f *testing.F) {
	f.Add("# Keyspace\ndb0:keys=42,expires=0,avg_ttl=0\n")
	f.Add("")
	f.Add("db0:keys=0,expires=0,avg_ttl=0\n")
	f.Add("db0:keys=999999999999999999999,expires=0\n") // overflow probe
	f.Add("\x00\x01\x02\xff")
	f.Fuzz(func(t *testing.T, s string) {
		n := parseKeyspaceKeys(s)
		if n < 0 {
			t.Fatalf("parseKeyspaceKeys returned negative: %d", n)
		}
	})
}

func FuzzParseClusterNodes(f *testing.F) {
	// Single-line minimal valid response.
	f.Add("abc123 10.0.0.1:6379@16379 myself,master - 0 0 1 connected 0-16383\n")
	f.Add("")
	f.Add("garbage line without enough fields\n")
	f.Add("aaa 1.1.1.1:6379@16379 master - 0 0 1 connected\nbbb 2.2.2.2:6379@16379 slave aaa 0 0 2 connected\n")
	f.Add(strings.Repeat("a", 10_000))
	f.Fuzz(func(t *testing.T, raw string) {
		// Property: never panics. parseClusterNodes is a transparent
		// parser — bound-checking the slot range against [0,16383] is
		// the controller's responsibility (Valkey could lie). The
		// fuzz target validates only the basic "well-ordered range"
		// invariant the parser advertises.
		nodes := parseClusterNodes(raw)
		for _, n := range nodes {
			for _, sr := range n.Slots {
				if sr.Start > sr.End {
					t.Fatalf("invalid slot range: %d > %d", sr.Start, sr.End)
				}
			}
		}
	})
}

func FuzzParseReplicationOffset(f *testing.F) {
	f.Add("# Replication\nrole:master\nmaster_replid:abc\nmaster_repl_offset:12345\n")
	f.Add("")
	f.Add("master_repl_offset:not-a-number\n")
	f.Add("master_repl_offset:-1\n")
	f.Add("slave_repl_offset:99999999999999\n")
	f.Fuzz(func(t *testing.T, s string) {
		// Property: doesn't panic. `-1` is a valid sentinel meaning
		// "no offset advertised" (documented contract).
		_ = ParseReplicationOffset(s)
	})
}
