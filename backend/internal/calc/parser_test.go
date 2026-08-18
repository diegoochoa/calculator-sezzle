package calc

import (
	"fmt"
	"strings"
	"testing"
)

// sexpr renders an AST as a prefix expression so precedence and associativity
// can be asserted on structure rather than inferred from a result.
func sexpr(node Node) string {
	switch n := node.(type) {
	case *NumberNode:
		return Format(n.Value, Precision)
	case *UnaryNode:
		return fmt.Sprintf("(%s %s)", n.Op, sexpr(n.X))
	case *PostfixNode:
		return fmt.Sprintf("(%s %s)", n.Op, sexpr(n.X))
	case *BinaryNode:
		return fmt.Sprintf("(%s %s %s)", n.Op, sexpr(n.Left), sexpr(n.Right))
	case *CallNode:
		parts := make([]string, 0, len(n.Args))
		for _, arg := range n.Args {
			parts = append(parts, sexpr(arg))
		}
		return fmt.Sprintf("(%s %s)", n.Name, strings.Join(parts, " "))
	default:
		return "?"
	}
}

func mustParse(t *testing.T, expression string) Node {
	t.Helper()
	node, err := Parse(expression, DefaultMaxLength, DefaultMaxDepth)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", expression, err)
	}
	return node
}

func TestParsePrecedenceAndAssociativity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expression string
		want       string
	}{
		{"1 + 2 * 3", "(+ 1 (* 2 3))"},
		{"1 * 2 + 3", "(+ (* 1 2) 3)"},
		{"1 - 2 - 3", "(- (- 1 2) 3)"},   // left associative
		{"2 ^ 3 ^ 2", "(^ 2 (^ 3 2))"},   // right associative
		{"-2 ^ 2", "(- (^ 2 2))"},        // negation binds looser than power
		{"2 ^ -3", "(^ 2 (- 3))"},        // an exponent may be negative
		{"(1 + 2) * 3", "(* (+ 1 2) 3)"}, // parentheses win
		{"50%", "(% 50)"},
		{"√9 + 1", "(+ (√ 9) 1)"},
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			t.Parallel()
			if got := sexpr(mustParse(t, tt.expression)); got != tt.want {
				t.Errorf("Parse(%q) = %s, want %s", tt.expression, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		wantCode   string
	}{
		{"empty", "", CodeEmpty},
		{"whitespace only", "   \t ", CodeEmpty},
		{"trailing operator", "2 +", CodeSyntax},
		{"leading operator", "* 2", CodeSyntax},
		{"juxtaposition", "2 3", CodeSyntax},
		{"empty parentheses", "()", CodeUnbalancedParen},
		{"unclosed parenthesis", "((1+2)", CodeUnbalancedParen},
		{"extra closing parenthesis", "1+2)", CodeSyntax},
		{"unknown name", "foo", CodeUnknownIdentifier},
		{"unknown function", "foo(1)", CodeUnknownIdentifier},
		{"no implicit multiplication", "2sqrt", CodeSyntax},
		{"function without parentheses", "sqrt", CodeSyntax},
		{"too many arguments", "sqrt(1, 2)", CodeWrongArity},
		{"unclosed call", "sqrt(1", CodeUnbalancedParen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tt.expression, DefaultMaxLength, DefaultMaxDepth)
			if err == nil {
				t.Fatalf("Parse(%q) error = nil, want %s", tt.expression, tt.wantCode)
			}
			if err.Code != tt.wantCode {
				t.Errorf("Parse(%q) code = %s, want %s (%s)", tt.expression, err.Code, tt.wantCode, err.Message)
			}
		})
	}
}

func TestParseRejectsOverlongExpression(t *testing.T) {
	t.Parallel()

	expression := strings.Repeat("1+", 200) + "1"
	_, err := Parse(expression, 32, DefaultMaxDepth)
	if err == nil || err.Code != CodeTooLong {
		t.Fatalf("Parse error = %v, want %s", err, CodeTooLong)
	}
}

// A recursive descent parser without a depth bound is a stack-overflow DoS, and
// a stack overflow in Go kills the process rather than the request.
func TestParseRejectsDeepNesting(t *testing.T) {
	t.Parallel()

	depth := 500
	expression := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)

	_, err := Parse(expression, len(expression)+1, DefaultMaxDepth)
	if err == nil || err.Code != CodeDepthExceeded {
		t.Fatalf("Parse error = %v, want %s", err, CodeDepthExceeded)
	}
}

func TestParseReportsPosition(t *testing.T) {
	t.Parallel()

	_, err := Parse("1 + foo", DefaultMaxLength, DefaultMaxDepth)
	if err == nil {
		t.Fatal("Parse error = nil, want an unknown identifier")
	}
	if err.Position != 4 {
		t.Errorf("position = %d, want 4 (the start of foo)", err.Position)
	}
}

func TestValidateDoesNotEvaluate(t *testing.T) {
	t.Parallel()

	// Syntactically valid, fatal to evaluate. Validate must not care.
	if err := Validate("1/0", Options{}); err != nil {
		t.Errorf("Validate(\"1/0\") = %v, want nil — validation is parse-only", err)
	}
	if err := Validate("1/", Options{}); err == nil {
		t.Error("Validate(\"1/\") = nil, want a syntax error")
	}
}
