package tokenhash_test

import (
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tokenhash"
)

func TestHash_DeterministicAndKnownVector(t *testing.T) {
	// Deterministic.
	if a, b := tokenhash.Hash("tok"), tokenhash.Hash("tok"); a != b {
		t.Fatalf("not deterministic: %q != %q", a, b)
	}
	// Distinct inputs → distinct hashes.
	if tokenhash.Hash("a") == tokenhash.Hash("b") {
		t.Fatal("collision on distinct inputs")
	}
	// Known SHA-256 vector for "abc" — must equal Postgres
	// encode(sha256('abc'::bytea),'hex') so the migration and app agree.
	const wantABC = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := tokenhash.Hash("abc"); got != wantABC {
		t.Fatalf("Hash(\"abc\") = %q, want %q", got, wantABC)
	}
	// Output is 64-char hex.
	if len(tokenhash.Hash("x")) != 64 {
		t.Fatalf("expected 64-char hex, got %d", len(tokenhash.Hash("x")))
	}
}
