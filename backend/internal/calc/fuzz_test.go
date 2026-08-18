package calc

import (
	"math"
	"testing"
)

// FuzzParse asserts the two properties a hand-written parser most often breaks:
// it never panics, and it never returns a success that the transport layer
// cannot encode. A panic here would take down the whole process, not just one
// request, so this is the cheapest insurance in the package.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"1 + 1",
		"2 + 3 * 4",
		"(((1)))",
		"2 ^ 3 ^ 2",
		"50%",
		"√9",
		"sqrt(16)",
		"abs(-1)",
		"1e400",
		"1.2.3",
		"))((",
		"",
		"        ",
		"-----1",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, expression string) {
		node, parseErr := Parse(expression, DefaultMaxLength, DefaultMaxDepth)
		if parseErr != nil {
			if node != nil {
				t.Fatalf("Parse(%q) returned both a node and the error %v", expression, parseErr)
			}
			return
		}
		if node == nil {
			t.Fatalf("Parse(%q) returned no node and no error", expression)
		}

		value, evalErr := Evaluate(node)
		if evalErr != nil {
			return
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("Evaluate(%q) = %v with no error; the value cannot be encoded as JSON",
				expression, value)
		}
	})
}
