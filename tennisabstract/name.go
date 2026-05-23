package tennisabstract

import (
	"regexp"
	"strings"
	"unicode"
)

var playerSlugPattern = regexp.MustCompile(`^[A-Za-z]+$`)

// PlayerSlug returns the Tennis Abstract player.cgi slug (p= query value).
//
// If name has no spaces and only ASCII letters, it is treated as an existing
// slug and returned trimmed unchanged (e.g. "DaniilMedvedev").
//
// Otherwise each whitespace-separated word is title-cased and concatenated
// (e.g. "Daniil Medvedev" → "DaniilMedvedev"). Particles like "de" are
// title-cased with the rest of the word ("de Minaur" → "DeMinaur"); hyphens
// and non-ASCII names are not special-cased yet.
func PlayerSlug(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.Contains(name, " ") && playerSlugPattern.MatchString(name) {
		return name
	}
	parts := strings.Fields(name)
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(titleWord(part))
	}
	return b.String()
}

func titleWord(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
