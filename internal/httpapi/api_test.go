package httpapi

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Tavern Keep":      "tavern-keep",
		"  Hello  World  ": "hello-world",
		"#main-game!!":     "main-game",
		"":                 "untitled",
		"---":              "untitled",
		"ÜBER":             "ber", // non-ascii stripped
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
