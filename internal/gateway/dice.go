package gateway

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Server-side dice (DH-1/D2): ONE implementation for every sender, human or
// persona. The server rolls with crypto/rand and the result is posted in-room,
// so nobody — person or model — can fudge a number and call it a roll.

var rollSpec = regexp.MustCompile(`^(?:(\d+))?d(\d+)\s*(?:([+-])\s*(\d+))?\s*(adv|dis)?$`)

type roll struct {
	n, sides, mod int
	mode          string // "" | adv | dis
}

// parseRoll accepts "2d6+3", "d20", "1d20 adv", "d%"… Empty spec = 1d20.
func parseRoll(spec string) (roll, error) {
	s := strings.ToLower(strings.TrimSpace(spec))
	if s == "" {
		s = "1d20"
	}
	s = strings.ReplaceAll(s, "d%", "d100")
	m := rollSpec.FindStringSubmatch(s)
	if m == nil {
		return roll{}, fmt.Errorf("can't read %q — try /roll 2d6+3 or /roll d20 adv", spec)
	}
	r := roll{n: 1, mode: m[5]}
	if m[1] != "" {
		r.n, _ = strconv.Atoi(m[1])
	}
	r.sides, _ = strconv.Atoi(m[2])
	if m[4] != "" {
		r.mod, _ = strconv.Atoi(m[4])
		if m[3] == "-" {
			r.mod = -r.mod
		}
	}
	switch {
	case r.n < 1 || r.n > 100:
		return roll{}, fmt.Errorf("dice count must be 1-100")
	case r.sides < 2 || r.sides > 1000:
		return roll{}, fmt.Errorf("die size must be 2-1000")
	case r.mode != "" && r.n != 1:
		return roll{}, fmt.Errorf("advantage/disadvantage applies to a single die")
	}
	return r, nil
}

func rollDie(sides int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		// crypto/rand failing is a platform catastrophe; a zero roll would be
		// silently unfair, so fail loud as the minimum (still in range).
		return 1
	}
	return int(v.Int64()) + 1
}

// rollCommand executes a /roll and renders the result body. The spec is echoed
// back so the room sees exactly what was rolled.
func rollCommand(spec string) (string, error) {
	r, err := parseRoll(spec)
	if err != nil {
		return "", err
	}
	display := strings.TrimSpace(strings.ToLower(spec))
	if display == "" {
		display = "1d20"
	}

	if r.mode != "" {
		a, b := rollDie(r.sides), rollDie(r.sides)
		kept := max(a, b)
		word := "keep high"
		if r.mode == "dis" {
			kept = min(a, b)
			word = "keep low"
		}
		total := kept + r.mod
		out := fmt.Sprintf("🎲 rolled `%s` → [%d, %d] %s %d", display, a, b, word, kept)
		if r.mod != 0 {
			out += fmt.Sprintf(" %+d", r.mod)
		}
		return out + fmt.Sprintf(" = **%d**", total), nil
	}

	rolls := make([]string, r.n)
	total := r.mod
	for i := range r.n {
		v := rollDie(r.sides)
		total += v
		rolls[i] = strconv.Itoa(v)
	}
	out := fmt.Sprintf("🎲 rolled `%s` → [%s]", display, strings.Join(rolls, ", "))
	if r.mod != 0 {
		out += fmt.Sprintf(" %+d", r.mod)
	}
	return out + fmt.Sprintf(" = **%d**", total), nil
}
