package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want             bool
	}{
		{"1.2.0", "1.1.0", true},
		{"1.1.0", "1.2.0", false},
		{"1.2.0", "1.2.0", false},
		{"1.10.0", "1.9.2", true},  // numeric, not lexicographic
		{"2.0.0", "1.9.9", true},
		{"1.2", "1.2.0", false},    // missing components pad as 0
		{"1.2.1", "1.2", true},
		{"abc", "1.0.0", false},    // malformed compares as 0.0.0
	}

	for _, tt := range tests {
		if got := isNewer(tt.latest, tt.current); got != tt.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestCheckSkipsDevVersion(t *testing.T) {
	for _, v := range []string{"", "dev"} {
		res, err := Check(v)
		if err != nil {
			t.Fatalf("Check(%q) error = %v, want nil (should never make a network call)", v, err)
		}
		if res.Available {
			t.Errorf("Check(%q) = %+v, want Available=false", v, res)
		}
	}
}
