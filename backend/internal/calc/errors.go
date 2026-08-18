package calc

import "fmt"

// Error codes raised by the engine. They are part of the public API contract:
// clients switch on them, so they must not change meaning.
const (
	CodeEmpty             = "EMPTY_EXPRESSION"
	CodeTooLong           = "EXPRESSION_TOO_LONG"
	CodeSyntax            = "SYNTAX_ERROR"
	CodeUnbalancedParen   = "UNBALANCED_PAREN"
	CodeUnknownIdentifier = "UNKNOWN_IDENTIFIER"
	CodeWrongArity        = "WRONG_ARITY"
	CodeDepthExceeded     = "DEPTH_EXCEEDED"
	CodeDivisionByZero    = "DIVISION_BY_ZERO"
	CodeDomain            = "DOMAIN_ERROR"
	CodeUndefined         = "UNDEFINED_RESULT"
	CodeOverflow          = "OVERFLOW"
)

// NoPosition marks an error that cannot be tied to an offset in the input.
const NoPosition = -1

// Error is every failure the engine can produce. Position is a 0-based rune
// offset into the submitted expression so a client can underline the problem.
type Error struct {
	Code     string
	Message  string
	Position int
}

func (e *Error) Error() string {
	if e.Position == NoPosition {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s at %d: %s", e.Code, e.Position, e.Message)
}

// at fills in the offset when the error was raised somewhere that did not know
// it — arithmetic helpers report *what* failed, the evaluator reports *where*.
func (e *Error) at(position int) *Error {
	if e == nil || e.Position != NoPosition {
		return e
	}
	clone := *e
	clone.Position = position
	return &clone
}

func newError(code, message string, position int) *Error {
	return &Error{Code: code, Message: message, Position: position}
}

// Errors raised without positional context; the evaluator attaches the offset.
func errDivisionByZero() *Error {
	return newError(CodeDivisionByZero, "Can't divide by zero", NoPosition)
}

func errDomain(message string) *Error {
	return newError(CodeDomain, message, NoPosition)
}

func errOverflow(message string) *Error {
	return newError(CodeOverflow, message, NoPosition)
}

func errUndefined(message string) *Error {
	return newError(CodeUndefined, message, NoPosition)
}
