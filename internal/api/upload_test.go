package api

import (
	"reflect"
	"testing"
)

func TestSplitTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"work", []string{"work"}},
		{"work,home", []string{"work", "home"}},
		{"work, home", []string{"work", "home"}},
		{"  work  ,  home  ", []string{"work", "home"}},
		{",,,", nil},
		{"work,,home", []string{"work", "home"}},
		{"a,b,c,d", []string{"a", "b", "c", "d"}},
	}

	for _, tt := range tests {
		got := splitTags(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitTags(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
