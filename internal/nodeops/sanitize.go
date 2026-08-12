package nodeops

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxLogLines   = 200
	MaxOutputSize = 32 * 1024
)

var (
	ansiPattern    = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
	secretPatterns = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer\s+)?)[^\s,;]+`), `${1}[REDACTED]`},
		{regexp.MustCompile(`(?i)\b(password|passwd|token|secret|credential|api[_-]?key)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`), `${1}${2}[REDACTED]`},
		{regexp.MustCompile(`(?i)\b(private(?:[_-]?key)?)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`), `${1}${2}[REDACTED]`},
		{regexp.MustCompile(`(?i)(hysteria2?://)[^@\s/]+@`), `${1}[REDACTED]@`},
		{regexp.MustCompile(`(?i)(vless://)[^@\s/]+@`), `${1}[REDACTED]@`},
		{regexp.MustCompile(`(?i)(https?://[^:/\s]+:)[^@/\s]+@`), `${1}[REDACTED]@`},
		{regexp.MustCompile(`(?i)(/sub/)[A-Za-z0-9._~-]{8,}`), `${1}[REDACTED]`},
		{regexp.MustCompile(`\bhys_[A-Za-z0-9_-]{16,}\b`), `[REDACTED]`},
		{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`), `[REDACTED]`},
	}
)

func SanitizeOutput(value string, maxLines, maxBytes int) string {
	if maxLines < 1 || maxLines > MaxLogLines {
		maxLines = MaxLogLines
	}
	if maxBytes < 1 || maxBytes > MaxOutputSize {
		maxBytes = MaxOutputSize
	}
	value = ansiPattern.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	var cleaned strings.Builder
	cleaned.Grow(min(len(value), maxBytes))
	for _, character := range value {
		if character == '\n' || character == '\t' || character >= 0x20 {
			cleaned.WriteRune(character)
		}
	}
	value = cleaned.String()
	for _, redaction := range secretPatterns {
		value = redaction.pattern.ReplaceAllString(value, redaction.replacement)
	}
	lines := strings.Split(value, "\n")
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines-1]
		truncated = true
	}
	value = strings.Join(lines, "\n")
	if len(value) > maxBytes {
		value = value[:maxBytes]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		truncated = true
	}
	value = strings.TrimSpace(value)
	if truncated {
		const suffix = "\n[truncated]"
		if len(value)+len(suffix) > maxBytes {
			value = value[:maxBytes-len(suffix)]
			for !utf8.ValidString(value) {
				value = value[:len(value)-1]
			}
			value = strings.TrimSpace(value)
		}
		value += suffix
	}
	return value
}

func SanitizeMessage(value string, maxBytes int) string {
	value = strings.ReplaceAll(SanitizeOutput(value, 1, maxBytes), "\n", " ")
	return strings.TrimSpace(value)
}
