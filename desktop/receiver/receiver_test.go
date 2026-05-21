package receiver

import (
	"testing"

	"github.com/lsariol/plop/desktop/config"
)

func TestResolveFolder(t *testing.T) {
	rules := []config.TagRule{
		{Tag: "work", Folder: "C:/Work"},
		{Tag: "photos", Folder: "C:/Pictures"},
	}
	const def = "C:/Default"

	tests := []struct {
		name   string
		tags   []string
		want   string
	}{
		{"exact match first rule", []string{"work"}, "C:/Work"},
		{"exact match second rule", []string{"photos"}, "C:/Pictures"},
		{"first match wins", []string{"work", "photos"}, "C:/Work"},
		{"case insensitive", []string{"WORK"}, "C:/Work"},
		{"case insensitive mixed", []string{"Photos"}, "C:/Pictures"},
		{"no match returns default", []string{"other"}, def},
		{"empty tags returns default", []string{}, def},
		{"nil tags returns default", nil, def},
		{"whitespace trimmed in tag", []string{"  work  "}, "C:/Work"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFolder(tt.tags, rules, def)
			if got != tt.want {
				t.Errorf("resolveFolder(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}
