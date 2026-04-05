package tokens

import "testing"

func TestEstimate(t *testing.T) {
	cases := []struct {
		input    int
		expected int
	}{
		{0, 0},
		{1, 0},
		{2, 1},
		{4, 1},
		{5, 1},
		{6, 2},
		{100, 25},
	}

	for _, c := range cases {
		got := Estimate(c.input)
		if got != c.expected {
			t.Errorf("Estimate(%d) = %d, want %d", c.input, got, c.expected)
		}
	}
}
