package smallprox

import (
	"testing"
)

func TestHasHeaderValuePart(t *testing.T) {
	for _, test := range hasHeaderValuePartTests {
		if HasAnyHeaderValuePart([]string{test[0]}, test[1]) != (test[2] == "t") {
			t.Errorf("Failed: %v", test)
		}
	}
}

var hasHeaderValuePartTests = [][]string{
	// if [0] has [1] then [2] is "t"
	[]string{"dog", "dog", "t"},
	[]string{"dogg", "dog", "f"},
	[]string{"dogg,dog", "dog", "t"},
	[]string{"dogg,dog,dogg", "dog", "t"},
	[]string{"dogg, dog", "dog", "t"},
	[]string{"dogg, dog, dogg", "dog", "t"},
	[]string{"cat,dog", "dog", "t"},
	[]string{"cat,dogg", "dog", "f"},
	[]string{"cat,dogg,dog", "dog", "t"},
	[]string{"cat,dogg,dog,dogg", "dog", "t"},
	[]string{"cat, dogg, dog", "dog", "t"},
	[]string{"cat, dogg, dog, dogg", "dog", "t"},
	[]string{"", "dog", "f"},
	[]string{"do", "dog", "f"},
	[]string{"do,g", "dog", "f"},
	[]string{", doggo", "dog", "f"},
	[]string{",doggo", "dog", "f"},
	[]string{" doggo", "dog", "f"},
}
