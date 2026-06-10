package gateway

import (
	"regexp"
	"strings"
	"testing"
)

func TestParseRoll(t *testing.T) {
	cases := []struct {
		spec string
		want roll
		err  bool
	}{
		{"2d6+3", roll{n: 2, sides: 6, mod: 3}, false},
		{"d20", roll{n: 1, sides: 20}, false},
		{"", roll{n: 1, sides: 20}, false}, // bare /roll = 1d20
		{"1d20 adv", roll{n: 1, sides: 20, mode: "adv"}, false},
		{"d20 dis", roll{n: 1, sides: 20, mode: "dis"}, false},
		{"4d8-2", roll{n: 4, sides: 8, mod: -2}, false},
		{"d%", roll{n: 1, sides: 100}, false},
		{"D12", roll{n: 1, sides: 12}, false}, // case-insensitive
		{"banana", roll{}, true},
		{"0d6", roll{}, true},
		{"101d6", roll{}, true},
		{"d1", roll{}, true},
		{"2d20 adv", roll{}, true}, // adv is single-die only
	}
	for _, c := range cases {
		got, err := parseRoll(c.spec)
		if c.err {
			if err == nil {
				t.Errorf("parseRoll(%q) should error, got %+v", c.spec, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseRoll(%q) = %+v, %v; want %+v", c.spec, got, err, c.want)
		}
	}
}

var totalRe = regexp.MustCompile(`= \*\*(-?\d+)\*\*$`)

func TestRollCommand_RangeAndShape(t *testing.T) {
	// Rolls are random; assert shape and range over repeats.
	for range 50 {
		out, err := rollCommand("2d6+3")
		if err != nil {
			t.Fatalf("roll: %v", err)
		}
		if !strings.Contains(out, "🎲 rolled `2d6+3`") || !totalRe.MatchString(out) {
			t.Fatalf("unexpected shape: %s", out)
		}
	}
	out, err := rollCommand("d20 adv")
	if err != nil || !strings.Contains(out, "keep high") {
		t.Fatalf("adv render: %s err=%v", out, err)
	}
	out, _ = rollCommand("d20 dis")
	if !strings.Contains(out, "keep low") {
		t.Fatalf("dis render: %s", out)
	}
	if _, err := rollCommand("nonsense"); err == nil {
		t.Fatal("bad spec must error")
	}
}
