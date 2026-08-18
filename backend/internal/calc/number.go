package calc

import (
	"math"
	"strconv"
	"strings"
)

// Precision-safe arithmetic, ported from the frontend's src/core/arithmetic.ts
// so both engines answer identically while they coexist.
//
// `0.1 + 0.2 == 0.30000000000000004` is unacceptable on a money surface, so
// operands are scaled to integers by a power of ten whenever that fits inside
// maxSafeInteger, and the operation runs there. Otherwise the result is rounded
// to Precision significant digits, which is all the display can show anyway.
//
// This is display-grade arithmetic, not ledger arithmetic. Financial operations
// (TVM, APR/APY, installment splits) want a decimal type instead.

// Precision is the number of significant digits kept for inexact results.
const Precision = 12

// maxSafeInteger matches JavaScript's Number.MAX_SAFE_INTEGER, deliberately:
// the frontend engine is bounded by it, and parity is the point.
const maxSafeInteger = 9007199254740991.0

// decimalPlaces counts digits after the point. The 'f' format never uses
// exponent notation, so the count is exact rather than guessed.
func decimalPlaces(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	text := strconv.FormatFloat(value, 'f', -1, 64)
	dot := strings.IndexByte(text, '.')
	if dot < 0 {
		return 0, true
	}
	return len(text) - dot - 1, true
}

// integerScale returns the power of ten that turns every value into a safe
// integer, and whether that is possible at all.
func integerScale(values ...float64) (float64, bool) {
	maxPlaces := 0
	for _, value := range values {
		places, ok := decimalPlaces(value)
		if !ok || places > Precision {
			return 0, false
		}
		if places > maxPlaces {
			maxPlaces = places
		}
	}

	scale := math.Pow(10, float64(maxPlaces))
	for _, value := range values {
		if math.Abs(value*scale) > maxSafeInteger {
			return 0, false
		}
	}
	return scale, true
}

// Round trims binary floating point noise, which only ever appears in the
// fractional part. Integers are returned untouched: rounding them to 12
// significant digits would corrupt exact results such as 2^53.
func Round(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	if value == math.Trunc(value) {
		return value
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(value, 'g', Precision, 64), 64)
	if err != nil {
		return value
	}
	return rounded
}

// Add returns a + b.
func Add(a, b float64) float64 {
	if scale, ok := integerScale(a, b); ok {
		return (math.Round(a*scale) + math.Round(b*scale)) / scale
	}
	return Round(a + b)
}

// Sub returns a - b.
func Sub(a, b float64) float64 {
	if scale, ok := integerScale(a, b); ok {
		return (math.Round(a*scale) - math.Round(b*scale)) / scale
	}
	return Round(a - b)
}

// Mul returns a * b.
func Mul(a, b float64) float64 {
	scaleA, okA := integerScale(a)
	scaleB, okB := integerScale(b)
	if !okA || !okB {
		return Round(a * b)
	}

	product := math.Round(a*scaleA) * math.Round(b*scaleB)
	if math.Abs(product) <= maxSafeInteger {
		return product / (scaleA * scaleB)
	}
	return Round(a * b)
}

// Div returns a / b.
func Div(a, b float64) (float64, *Error) {
	if b == 0 {
		return 0, errDivisionByZero()
	}
	if scale, ok := integerScale(a, b); ok {
		return Round(math.Round(a*scale) / math.Round(b*scale)), nil
	}
	return Round(a / b), nil
}

// Pow returns base raised to exponent.
func Pow(base, exponent float64) (float64, *Error) {
	if base == 0 && exponent < 0 {
		return 0, errDivisionByZero()
	}
	// A negative base only has a real power for integer exponents.
	if base < 0 && exponent != math.Trunc(exponent) {
		return 0, errDomain("A negative base has no real power for a fractional exponent")
	}
	return Round(math.Pow(base, exponent)), nil
}

// guard converts a non-finite result into a typed error. It runs after every
// operation because NaN and ±Inf are not representable in JSON and would
// otherwise surface as a 500.
func guard(value float64) (float64, *Error) {
	switch {
	case math.IsNaN(value):
		return 0, errUndefined("The result is undefined")
	case math.IsInf(value, 0):
		return 0, errOverflow("The result is out of range")
	default:
		return value, nil
	}
}

// Format renders a result for display: plain decimal where that is readable,
// exponent notation only when the magnitude demands it.
func Format(value float64, precision int) string {
	if value == 0 {
		return "0" // collapses -0
	}
	if precision <= 0 || precision > 17 {
		precision = Precision
	}
	if value == math.Trunc(value) && math.Abs(value) < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strconv.FormatFloat(value, 'g', precision, 64)
}
