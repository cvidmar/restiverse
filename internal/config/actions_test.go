package config

import "testing"

func TestBuildCommandQuotesNumberedPaths(t *testing.T) {
	action := Action{Command: "jd {filename1} {filename2}"}
	got, err := action.BuildCommand([]string{"a file.body", "value's.body"})
	if err != nil {
		t.Fatal(err)
	}
	want := `jd 'a file.body' 'value'\''s.body'`
	if got != want {
		t.Fatalf("BuildCommand = %q, want %q", got, want)
	}
}
