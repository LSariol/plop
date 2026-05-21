package client

import "testing"

func TestToWSURL(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"http://localhost:8080", "ws://localhost:8080", false},
		{"https://plop.example.com", "wss://plop.example.com", false},
		{"http://plop.example.com/extra/path", "ws://plop.example.com/extra/path", false},
		{"ftp://bad.example.com", "", true},
		{"not-a-url", "", true},
	}

	for _, tt := range tests {
		got, err := toWSURL(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("toWSURL(%q): want error, got nil (result=%q)", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("toWSURL(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("toWSURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
