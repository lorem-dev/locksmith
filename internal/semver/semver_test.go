package semver

import "testing"

func TestParse_Valid(t *testing.T) {
	cases := []struct {
		in              string
		major, minor, p int
	}{
		{"0.1.0", 0, 1, 0},
		{"1.0.0", 1, 0, 0},
		{"10.20.30", 10, 20, 30},
	}
	for _, c := range cases {
		v, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", c.in, err)
			continue
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.p {
			t.Errorf("Parse(%q) = %+v, want {%d %d %d}", c.in, v, c.major, c.minor, c.p)
		}
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, in := range []string{"v1.0.0", "1.0.0-rc1", "1.0", "abc", "", "1.0.0.0", "1.-1.0", "a.b.c"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) expected error", in)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.1.0", "1.2.0", -1},
		{"1.2.0", "1.1.0", 1},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
	}
	for _, c := range cases {
		va, _ := Parse(c.a)
		vb, _ := Parse(c.b)
		if got := va.Compare(vb); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLessOrEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
	}
	for _, c := range cases {
		va, _ := Parse(c.a)
		vb, _ := Parse(c.b)
		if got := va.LessOrEqual(vb); got != c.want {
			t.Errorf("%q.LessOrEqual(%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestGreaterOrEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "1.0.0", true},
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
	}
	for _, c := range cases {
		va, _ := Parse(c.a)
		vb, _ := Parse(c.b)
		if got := va.GreaterOrEqual(vb); got != c.want {
			t.Errorf("%q.GreaterOrEqual(%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
