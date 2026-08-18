package calc

import (
	"math"
	"testing"
)

func evaluate(t *testing.T, expression string) float64 {
	t.Helper()
	result, err := Calculate(expression, Options{})
	if err != nil {
		t.Fatalf("Calculate(%q) error = %v", expression, err)
	}
	return result.Value
}

func TestEvaluateArithmetic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expression string
		want       float64
	}{
		{"1 + 1", 2},
		{"0.1 + 0.2", 0.3},
		{"19.99 + 0.01", 20},
		{"1.5 - 1.2", 0.3},
		{"100 - 99.99", 0.01},
		{"1.1 * 3", 3.3},
		{"0.07 * 100", 7},
		{"10 / 4", 2.5},
		{"0.3 / 0.1", 3},
		{"1 / 3", 0.333333333333},

		// Precedence, which is the whole reason for a parser. The original
		// left-to-right engine answered 20.
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"10 - 2 - 3", 5},  // left associative
		{"2 ^ 3 ^ 2", 512}, // right associative
		{"-2 ^ 2", -4},     // negation binds looser than power
		{"2 ^ -3", 0.125},  // negative exponent
		{"2 ^ 53", 9007199254740992},
		{"2 ^ 0.5", 1.41421356237},

		{"50%", 0.5},
		{"200 * 10%", 20},
		{"√9", 3},
		{"√9 + 1", 4},
		{"2 * √9", 6},
		{"sqrt(16)", 4},
		{"abs(-3.5)", 3.5},
		{"--5", 5},
		{"+3", 3},
		{"1e3 + 1", 1001},

		// The glyphs the UI renders are accepted as typed.
		{"6 ÷ 2 × 3", 9},
		{"5 − 2", 3},
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			t.Parallel()
			if got := evaluate(t, tt.expression); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.expression, got, tt.want)
			}
		})
	}
}

func TestEvaluateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		wantCode   string
	}{
		{"division by zero", "1/0", CodeDivisionByZero},
		{"division by a zero subexpression", "1/(2-2)", CodeDivisionByZero},
		{"zero to a negative power", "0 ^ -1", CodeDivisionByZero},
		{"square root of a negative", "sqrt(-1)", CodeDomain},
		{"radical of a negative", "√-1", CodeDomain},
		{"fractional power of a negative base", "(-8) ^ 0.5", CodeDomain},
		{"power overflow", "10 ^ 400", CodeOverflow},
		{"multiplication overflow", "1e308 * 1e308", CodeOverflow},
		{"percent of nothing is still fine", "0%", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Calculate(tt.expression, Options{})
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("Calculate(%q) error = %v, want success", tt.expression, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Calculate(%q) error = nil, want %s", tt.expression, tt.wantCode)
			}
			if err.Code != tt.wantCode {
				t.Errorf("Calculate(%q) code = %s, want %s (%s)", tt.expression, err.Code, tt.wantCode, err.Message)
			}
		})
	}
}

// Runtime failures must point at the operator that failed, not at the start of
// the expression, so a client can underline it.
func TestEvaluateErrorPosition(t *testing.T) {
	t.Parallel()

	_, err := Calculate("100 + 1/0", Options{})
	if err == nil {
		t.Fatal("error = nil, want a division by zero")
	}
	if err.Position != 7 {
		t.Errorf("position = %d, want 7 (the slash)", err.Position)
	}
}

// No result may reach the transport layer as NaN or ±Inf: neither is
// representable in JSON, and both would surface as a 500.
func TestEvaluateNeverReturnsNonFinite(t *testing.T) {
	t.Parallel()

	expressions := []string{
		"10 ^ 400", "1/0", "0/0", "0 ^ -1", "1e308 * 1e308", "-1e308 * 1e308",
	}

	for _, expression := range expressions {
		result, err := Calculate(expression, Options{})
		if err == nil {
			t.Errorf("Calculate(%q) succeeded with %v, want a typed error", expression, result.Value)
			continue
		}
		if math.IsNaN(result.Value) || math.IsInf(result.Value, 0) {
			t.Errorf("Calculate(%q) leaked %v alongside the error", expression, result.Value)
		}
	}
}

func TestCalculateFormatsResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expression string
		want       string
	}{
		{"2 + 2", "4"},
		{"0.1 + 0.2", "0.3"},
		{"1/3", "0.333333333333"},
		{"2 ^ 53", "9007199254740992"},
		{"0 - 0", "0"},
	}

	for _, tt := range tests {
		result, err := Calculate(tt.expression, Options{})
		if err != nil {
			t.Fatalf("Calculate(%q) error = %v", tt.expression, err)
		}
		if result.Formatted != tt.want {
			t.Errorf("Calculate(%q).Formatted = %q, want %q", tt.expression, result.Formatted, tt.want)
		}
	}
}

func TestCalculateHonoursPrecision(t *testing.T) {
	t.Parallel()

	result, err := Calculate("1/3", Options{Precision: 4})
	if err != nil {
		t.Fatalf("Calculate error = %v", err)
	}
	if result.Formatted != "0.3333" {
		t.Errorf("Formatted = %q, want 0.3333", result.Formatted)
	}
}
