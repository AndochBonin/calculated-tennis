package tennisabstract

import (
	"strings"
	"unicode"
)

// surnameParticles are tokens that start a multi-word surname in display names.
var surnameParticles = map[string]struct{}{
	"de": {}, "del": {}, "della": {}, "da": {}, "dos": {}, "du": {},
	"van": {}, "von": {}, "le": {}, "la": {}, "st": {}, "st.": {},
}

// playerMatchKey identifies a player for cross-dataset match joins.
type playerMatchKey struct {
	last   string // normalized surname
	initial string
}

func playerKeyFromSackmannName(name string) playerMatchKey {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return playerMatchKey{}
	}
	if len(parts) == 1 {
		return playerMatchKey{last: normalizePlayerKeyPart(parts[0]), initial: ""}
	}
	ini := ""
	if r := []rune(parts[0]); len(r) > 0 {
		ini = strings.ToLower(string(r[0]))
	}
	surname := sackmannSurname(parts)
	return playerMatchKey{last: normalizePlayerKeyPart(surname), initial: ini}
}

func sackmannSurname(parts []string) string {
	if len(parts) <= 1 {
		return parts[0]
	}
	particleAt := -1
	for i := 1; i < len(parts); i++ {
		if _, ok := surnameParticles[strings.ToLower(parts[i])]; ok {
			particleAt = i
			break
		}
	}
	if particleAt >= 0 {
		return strings.Join(parts[particleAt:], " ")
	}
	return parts[len(parts)-1]
}

// playerKeyFromOddsName parses tennis-data style names (e.g. "Medvedev D.", "O Connell C.").
func playerKeyFromOddsName(name string) playerMatchKey {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return playerMatchKey{}
	}
	lastPart := parts[len(parts)-1]
	if strings.HasSuffix(lastPart, ".") {
		ini := strings.TrimSuffix(lastPart, ".")
		ini = strings.ToLower(ini)
		if ini != "" {
			ini = ini[:1]
		}
		last := strings.Join(parts[:len(parts)-1], " ")
		return playerMatchKey{last: normalizePlayerKeyPart(last), initial: ini}
	}
	ini := ""
	if len(parts) > 1 {
		if r := []rune(parts[1]); len(r) > 0 {
			ini = strings.ToLower(string(r[0]))
		}
	}
	return playerMatchKey{last: normalizePlayerKeyPart(parts[0]), initial: ini}
}

func normalizePlayerKeyPart(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func matchPlayerKeys(a, b playerMatchKey) bool {
	if a.last == "" || b.last == "" {
		return false
	}
	if a.last != b.last {
		return false
	}
	if a.initial != "" && b.initial != "" && a.initial != b.initial {
		return false
	}
	return true
}
