package smallprox

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/exp/errors"
)

func TestNoscript(t *testing.T) {
	for _, test := range noscriptTests {
		input := test[0]
		t.Logf("Input:  `%s`", input)
		expect := test[1]
		outputbuf := &strings.Builder{}
		inputbuf := bytes.NewBufferString(input)
		err := noscriptStreamer(inputbuf, outputbuf)
		if err != nil {
			t.Error(err)
			continue
		}
		output := outputbuf.String()
		t.Logf("Output: `%s`", output)
		if expect != output {
			t.Logf("Expect: `%s`", expect)
			t.Error(errors.New("Did not get expected output"))
			continue
		}
	}
}

var noscriptTests = [][]string{
	[]string{" foo  bar ", " foo  bar "},
	[]string{" <b > bold </b > ", " <b > bold </b > "},
	[]string{" <button onclick='stuff()'>foo</button> ", " <button>foo</button> "},
	[]string{" <script/> foo ", "  foo "},
	[]string{" <script>x</script> foo ", "  foo "},
	[]string{" <script> <script/> x() </script> foo ", "  foo "},
	[]string{" <script> a() <script> b() </script> x() </script> foo ", "  x()  foo "},
	[]string{" foo <noscript>x</noscript> bar ", " foo <div data-from-noscript=true>x</div> bar "},
	[]string{`<a href="/asdf">foo</a>`, `<a href="/asdf">foo</a>`},
	[]string{`<a href="javascript:x()">foo</a>`, `<a href=#noscript>foo</a>`},
}
