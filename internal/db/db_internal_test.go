package db

import "testing"

func TestIsPostgres(t *testing.T) {
	tests := []struct {
		dsn  string
		want bool
	}{
		{"postgres://user:pass@localhost/cotests", true},
		{"postgresql://user:pass@localhost/cotests", true},
		{"host=localhost user=cotests", true},
		{"cotests.db", false},
		{":memory:", false},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			if got := isPostgres(tt.dsn); got != tt.want {
				t.Fatalf("isPostgres(%q) = %t, want %t", tt.dsn, got, tt.want)
			}
		})
	}
}
