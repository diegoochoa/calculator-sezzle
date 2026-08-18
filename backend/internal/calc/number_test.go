package calc

import (
	"math"
	"testing"
)

func TestAddSubAvoidFloatDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"the canonical case", Add(0.1, 0.2), 0.3},
		{"cents", Add(19.99, 0.01), 20},
		{"subtraction drift", Sub(1.5, 1.2), 0.3},
		{"cents back out", Sub(100, 99.99), 0.01},
		{"unscalable operands fall back to rounding", Add(1e21, 1), 1e21},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestMul(t *testing.T) {
	t.Parallel()

	tests := []struct{ a, b, want float64 }{
		{1.1, 3, 3.3},
		{0.07, 100, 7},
		{2.5, 4, 10},
		{1e11, 1e11, 1e22}, // beyond the integer path, rounded instead
	}

	for _, tt := range tests {
		if got := Mul(tt.a, tt.b); got != tt.want {
			t.Errorf("Mul(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDiv(t *testing.T) {
	t.Parallel()

	tests := []struct{ a, b, want float64 }{
		{10, 4, 2.5},
		{0.3, 0.1, 3},
		{1, 3, 0.333333333333},
	}

	for _, tt := range tests {
		got, err := Div(tt.a, tt.b)
		if err != nil {
			t.Fatalf("Div(%v, %v) error = %v", tt.a, tt.b, err)
		}
		if got != tt.want {
			t.Errorf("Div(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}

	if _, err := Div(1, 0); err == nil || err.Code != CodeDivisionByZero {
		t.Errorf("Div(1, 0) error = %v, want %s", err, CodeDivisionByZero)
	}
}

func TestPow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		base, exponent, want float64
	}{
		{2, 10, 1024},
		{2, 0.5, 1.41421356237},
		{2, -2, 0.25},
		{2, 53, 9007199254740992}, // exact integers must survive rounding
	}

	for _, tt := range tests {
		got, err := Pow(tt.base, tt.exponent)
		if err != nil {
			t.Fatalf("Pow(%v, %v) error = %v", tt.base, tt.exponent, err)
		}
		if got != tt.want {
			t.Errorf("Pow(%v, %v) = %v, want %v", tt.base, tt.exponent, got, tt.want)
		}
	}

	if _, err := Pow(-8, 0.5); err == nil || err.Code != CodeDomain {
		t.Errorf("Pow(-8, 0.5) error = %v, want %s", err, CodeDomain)
	}
	if _, err := Pow(0, -1); err == nil || err.Code != CodeDivisionByZero {
		t.Errorf("Pow(0, -1) error = %v, want %s", err, CodeDivisionByZero)
	}
}

func TestRoundKeepsIntegersExact(t *testing.T) {
	t.Parallel()

	if got := Round(9007199254740992); got != 9007199254740992 {
		t.Errorf("Round(2^53) = %v, want it untouched", got)
	}
	if got := Round(0.1 + 0.2); got != 0.3 {
		t.Errorf("Round(0.1+0.2) = %v, want 0.3", got)
	}
	if got := Round(math.NaN()); !math.IsNaN(got) {
		t.Errorf("Round(NaN) = %v, want NaN passed through for the guard to catch", got)
	}
}

func TestGuard(t *testing.T) {
	t.Parallel()

	if _, err := guard(math.NaN()); err == nil || err.Code != CodeUndefined {
		t.Errorf("guard(NaN) error = %v, want %s", err, CodeUndefined)
	}
	if _, err := guard(math.Inf(1)); err == nil || err.Code != CodeOverflow {
		t.Errorf("guard(+Inf) error = %v, want %s", err, CodeOverflow)
	}
	if value, err := guard(42); err != nil || value != 42 {
		t.Errorf("guard(42) = %v, %v", value, err)
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value float64
		want  string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"}, // negative zero collapses
		{1024, "1024"},
		{9007199254740992, "9007199254740992"},
		{0.3, "0.3"},
		{-1.5, "-1.5"},
		{1e21, "1e+21"},
		{1.0 / 3.0, "0.333333333333"},
	}

	for _, tt := range tests {
		if got := Format(tt.value, Precision); got != tt.want {
			t.Errorf("Format(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestDecimalPlaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value  float64
		want   int
		wantOK bool
	}{
		{1, 0, true},
		{1.25, 2, true},
		{0.001, 3, true},
		{math.NaN(), 0, false},
		{math.Inf(1), 0, false},
	}

	for _, tt := range tests {
		got, ok := decimalPlaces(tt.value)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("decimalPlaces(%v) = (%d, %t), want (%d, %t)", tt.value, got, ok, tt.want, tt.wantOK)
		}
	}
}
