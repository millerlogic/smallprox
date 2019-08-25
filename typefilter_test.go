package smallprox

import "testing"

func TestTypeFilter(t *testing.T) {
	filter := []string{"a/b", "c/d", "e", "f"}
	t.Logf("Filter list: %+v", filter)
	for _, x := range [][]string{
		[]string{"a/b", "hello.a"},
		[]string{"c/d", "asdf"},
		[]string{"c/d+xml", "asdf"},
		[]string{"c/d; charset=utf-8", "asdf"},
	} {
		if !inTypeFilter(x[0], x[1], filter) {
			t.Errorf("Expected %+v to match filter list", x)
		}
	}
	for _, x := range [][]string{
		[]string{"e/f", "hello.a"},
		[]string{"a/bb", "hello.a"},
		[]string{"aa/b", "hello.a"},
		[]string{"a/d", "asdf"},
	} {
		if inTypeFilter(x[0], x[1], filter) {
			t.Errorf("Expected %+v NOT to match filter list", x)
		}
	}
}
