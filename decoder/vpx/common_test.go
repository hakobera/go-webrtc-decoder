package vpx

import (
	"testing"
)

func TestMin(t *testing.T) {
	cases := []struct {
		a, b, expected int
	}{
		{0, 0, 0},
		{0, 1, 0},
		{1, 0, 0},
		{1, 2, 1},
		{2, 1, 1},
	}

	for _, c := range cases {
		ret := min(c.a, c.b)
		if ret != c.expected {
			t.Errorf("min(%d, %d) should return %d but got %d", c.a, c.b, c.expected, ret)
		}
	}
}
