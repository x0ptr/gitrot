package utils

import "testing"

func TestFormatAuthorName(t *testing.T) {
	name := "Max Mustermann"
	if got := FormatAuthorName(name, false); got != name {
		t.Fatalf("expected original name, got %q", got)
	}

	hiddenA := FormatAuthorName(name, true)
	hiddenB := FormatAuthorName(name, true)
	if hiddenA != hiddenB {
		t.Fatalf("expected deterministic obfuscation, got %q and %q", hiddenA, hiddenB)
	}
	if len(hiddenA) != len("usr-12345678") || hiddenA[:4] != "usr-" {
		t.Fatalf("unexpected obfuscated format: %q", hiddenA)
	}
}
