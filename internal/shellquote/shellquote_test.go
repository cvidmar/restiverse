package shellquote

import "testing"

func TestQuote(t *testing.T) {
	for input, want := range map[string]string{
		"":        "''",
		"simple":  "'simple'",
		"a b":     "'a b'",
		"value's": `'value'\''s'`,
	} {
		if got := Quote(input); got != want {
			t.Errorf("Quote(%q) = %q, want %q", input, got, want)
		}
	}
}
