package runner

import (
	"bytes"
	"testing"
)

func TestCanonicalRulesSchemaAndDigest(t *testing.T) {
	data, hash, err := canonicalRules([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := validateSnapshot(data, hash); err != nil || !bytes.Equal(data, actual) {
		t.Fatal(err)
	}
	if _, err := validateSnapshot(data, "tampered"); err == nil {
		t.Fatal("bad checksum accepted")
	}
	for _, raw := range []string{`null`, `{"grid_width":1000}`, `{"mobs":null}`, `{"mobs":{"count":1.5}}`, `{"match_rules":{"duration":19}}`, `{"xp":{"growth":4}}`, `{"match_rules":{"zone":null}}`} {
		if _, _, err := canonicalRules([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	other, otherHash, err := canonicalRules([]byte(`{"match_rules":{"zone":false}}`))
	if err != nil || hash == otherHash || bytes.Equal(data, other) {
		t.Fatal("changed rules must change digest")
	}
	rounded, roundedHash, err := canonicalRules([]byte(`{"xp":{"growth":1.333333333333}}`))
	if err != nil {
		t.Fatal(err)
	}
	_, roundTripHash, err := canonicalRules(rounded)
	if err != nil || roundTripHash != roundedHash {
		t.Fatal("float32 normalization must be idempotent")
	}
}
