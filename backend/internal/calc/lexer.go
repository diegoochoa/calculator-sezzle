package calc

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenNumber
	tokenIdent
	tokenPlus
	tokenMinus
	tokenStar
	tokenSlash
	tokenCaret
	tokenPercent
	tokenRadical
	tokenLParen
	tokenRParen
	tokenComma
)

func (k tokenKind) String() string {
	switch k {
	case tokenEOF:
		return "end of expression"
	case tokenNumber:
		return "number"
	case tokenIdent:
		return "name"
	case tokenPlus:
		return `"+"`
	case tokenMinus:
		return `"-"`
	case tokenStar:
		return `"*"`
	case tokenSlash:
		return `"/"`
	case tokenCaret:
		return `"^"`
	case tokenPercent:
		return `"%"`
	case tokenRadical:
		return `"√"`
	case tokenLParen:
		return `"("`
	case tokenRParen:
		return `")"`
	case tokenComma:
		return `","`
	default:
		return "token"
	}
}

type token struct {
	kind  tokenKind
	text  string
	value float64
	pos   int
}

// runeTokens maps single characters to their token kind. The Unicode entries
// exist because the frontend renders those glyphs on its keys, so an expression
// copied out of the UI lexes without translation.
var runeTokens = map[rune]tokenKind{
	'+': tokenPlus,
	'-': tokenMinus,
	'−': tokenMinus, // U+2212 minus sign
	'–': tokenMinus, // en dash, a common paste artefact
	'*': tokenStar,
	'×': tokenStar,
	'·': tokenStar,
	'/': tokenSlash,
	'÷': tokenSlash,
	'^': tokenCaret,
	'%': tokenPercent,
	'√': tokenRadical,
	'(': tokenLParen,
	')': tokenRParen,
	',': tokenComma,
}

// tokenize converts an expression into tokens. Positions are rune offsets, so
// they stay meaningful for clients indexing a JavaScript string of the same
// text after accounting for surrogate pairs (none of our glyphs use them).
func tokenize(expression string) ([]token, *Error) {
	runes := []rune(expression)
	tokens := make([]token, 0, len(runes)/2+1)

	for i := 0; i < len(runes); {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			i++

		case isDigit(r) || r == '.':
			tok, next, err := scanNumber(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next

		case isIdentStart(r):
			start := i
			for i < len(runes) && isIdentPart(runes[i]) {
				i++
			}
			name := strings.ToLower(string(runes[start:i]))
			tokens = append(tokens, token{kind: tokenIdent, text: name, pos: start})

		default:
			if kind, ok := runeTokens[r]; ok {
				tokens = append(tokens, token{kind: kind, text: string(r), pos: i})
				i++
				continue
			}
			return nil, newError(CodeSyntax, "Unexpected character "+strconv.QuoteRune(r), i)
		}
	}

	tokens = append(tokens, token{kind: tokenEOF, pos: len(runes)})
	return tokens, nil
}

// scanNumber reads a decimal literal with an optional exponent. The exponent is
// only consumed when it is followed by digits, so `2*e` keeps `e` as the
// constant while `2e5` is a single literal.
func scanNumber(runes []rune, start int) (token, int, *Error) {
	i := start
	for i < len(runes) && (isDigit(runes[i]) || runes[i] == '.') {
		i++
	}

	if i < len(runes) && (runes[i] == 'e' || runes[i] == 'E') {
		j := i + 1
		if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
			j++
		}
		if j < len(runes) && isDigit(runes[j]) {
			for j < len(runes) && isDigit(runes[j]) {
				j++
			}
			i = j
		}
	}

	text := string(runes[start:i])
	value, err := strconv.ParseFloat(text, 64)
	switch {
	case err != nil && strings.Contains(err.Error(), "value out of range"):
		return token{}, start, newError(CodeOverflow, "The number "+text+" is out of range", start)
	case err != nil:
		return token{}, start, newError(CodeSyntax, "Invalid number "+strconv.Quote(text), start)
	case math.IsInf(value, 0):
		return token{}, start, newError(CodeOverflow, "The number "+text+" is out of range", start)
	}

	return token{kind: tokenNumber, text: text, value: value, pos: start}, i, nil
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }

func isIdentPart(r rune) bool { return isIdentStart(r) || isDigit(r) }
