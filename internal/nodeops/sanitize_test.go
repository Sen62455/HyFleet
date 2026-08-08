package nodeops

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeOutputBoundsAndRedacts(t *testing.T) {
	input := "\x1b[31mauthorization: Bearer should-not-remain\x1b[0m\n" +
		"password=another-value\n" +
		"hysteria2://credential-value@example.test:443\n" +
		"subscription=https://panel.example.test/sub/hys_abcdefghijklmnopqrstuvwxyz/clash\n" +
		"control:\x00removed\n" + strings.Repeat("line\n", 250)
	got := SanitizeOutput(input, 20, 160)
	for _, forbidden := range []string{
		"should-not-remain", "another-value", "credential-value",
		"hys_abcdefghijklmnopqrstuvwxyz", "\x1b", "\x00",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("SanitizeOutput() retained %q in %q", forbidden, got)
		}
	}
	if len(got) > 160 || strings.Count(got, "\n") >= 20 || !strings.Contains(got, "[truncated]") {
		t.Fatalf("SanitizeOutput() bounds not applied: bytes=%d lines=%d output=%q", len(got), strings.Count(got, "\n")+1, got)
	}
}

func TestSanitizeOutputPreservesValidUTF8WhenTruncated(t *testing.T) {
	got := SanitizeOutput(strings.Repeat("状态", 100), 10, 31)
	if !utf8.ValidString(got) || len(got) > 31 {
		t.Fatalf("SanitizeOutput() produced invalid or oversized output: %q", got)
	}
}

func TestSanitizeOutputRedactsSubscriptionTokens(t *testing.T) {
	const token = "hys_abcdefghijklmnopqrstuvwxyz"
	got := SanitizeOutput(
		"request=https://panel.example.test/sub/"+token+"/clash\nbare="+token,
		10,
		512,
	)
	if strings.Contains(got, token) ||
		!strings.Contains(got, "/sub/[REDACTED]/clash") ||
		!strings.Contains(got, "bare=[REDACTED]") {
		t.Fatalf("SanitizeOutput() did not redact subscription tokens: %q", got)
	}
}
