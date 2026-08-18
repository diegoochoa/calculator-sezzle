package calc

import (
	"fmt"
	"math"
	"sort"
)

// The function set is deliberately small: square root and absolute value are
// the only named functions the calculator exposes. Trigonometry, logarithms,
// hyperbolics and factorials were removed along with the scientific keypad —
// the brief asks for the four operations, and every one of those extras brought
// a judgement call (angle units, domain edges) that the requirements never
// needed answered.
//
// The machinery around this registry is unchanged, so adding a function back is
// one entry in the map plus its catalog text.

// FunctionDef describes one callable function.
type FunctionDef struct {
	Name    string
	MinArgs int
	MaxArgs int
	Summary string
	// Domain documents the accepted inputs, surfaced by the catalog endpoint.
	Domain string

	apply func(args []float64) (float64, *Error)
}

// unary adapts a single-argument implementation.
func unary(fn func(float64) (float64, *Error)) func([]float64) (float64, *Error) {
	return func(args []float64) (float64, *Error) { return fn(args[0]) }
}

var functions = map[string]FunctionDef{
	"sqrt": {
		Name: "sqrt", MinArgs: 1, MaxArgs: 1,
		Summary: "Square root",
		Domain:  "[0, ∞)",
		apply: unary(func(x float64) (float64, *Error) {
			if x < 0 {
				return 0, errDomain("The square root of a negative number is not real")
			}
			return Round(math.Sqrt(x)), nil
		}),
	},
	"abs": {
		Name: "abs", MinArgs: 1, MaxArgs: 1,
		Summary: "Absolute value",
		apply:   unary(func(x float64) (float64, *Error) { return math.Abs(x), nil }),
	},
}

// arityMessage explains a wrong argument count in the terms a user typed it.
func arityMessage(def FunctionDef, got int) string {
	if def.MinArgs == def.MaxArgs {
		return fmt.Sprintf("%s takes %d argument(s) but got %d", def.Name, def.MinArgs, got)
	}
	return fmt.Sprintf("%s takes between %d and %d arguments but got %d",
		def.Name, def.MinArgs, def.MaxArgs, got)
}

// FunctionNames lists every function, sorted, for documentation and tests.
func FunctionNames() []string {
	names := make([]string, 0, len(functions))
	for name := range functions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
