// Package calc is the calculation engine: lexer, parser, evaluator and
// precision-safe arithmetic. It has no knowledge of HTTP, which is what makes
// it exhaustively table-testable and reusable outside this server.
package calc

// Options bound what the engine will accept. Zero values fall back to the
// defaults, so callers can pass a partially filled struct.
type Options struct {
	// MaxLength is the expression limit in runes.
	MaxLength int
	// MaxDepth bounds parser recursion.
	MaxDepth int
	// Precision is the significant digits used when formatting.
	Precision int
}

// Defaults mirror the server's own defaults so the engine is usable standalone.
const (
	DefaultMaxLength = 256
	DefaultMaxDepth  = 32
)

func (o Options) withDefaults() Options {
	if o.MaxLength <= 0 {
		o.MaxLength = DefaultMaxLength
	}
	if o.MaxDepth <= 0 {
		o.MaxDepth = DefaultMaxDepth
	}
	if o.Precision <= 0 {
		o.Precision = Precision
	}
	return o
}

// Result is one evaluated expression. Formatted is the display string, so the
// client does not have to re-derive the engine's rounding rules.
type Result struct {
	Value     float64
	Formatted string
}

// Calculate parses and evaluates an expression.
func Calculate(expression string, opts Options) (Result, *Error) {
	o := opts.withDefaults()

	node, err := Parse(expression, o.MaxLength, o.MaxDepth)
	if err != nil {
		return Result{}, err
	}

	value, err := Evaluate(node)
	if err != nil {
		return Result{}, err
	}

	return Result{Value: value, Formatted: Format(value, o.Precision)}, nil
}

// Validate parses without evaluating, so a client can check syntax as the user
// types without triggering division by zero or a slow computation.
func Validate(expression string, opts Options) *Error {
	o := opts.withDefaults()
	_, err := Parse(expression, o.MaxLength, o.MaxDepth)
	return err
}
