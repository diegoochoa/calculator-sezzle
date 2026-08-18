package calc

import "testing"

func kinds(tokens []token) []tokenKind {
	out := make([]tokenKind, 0, len(tokens))
	for _, tok := range tokens {
		out = append(out, tok.kind)
	}
	return out
}

func TestTokenizeShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		want       []tokenKind
	}{
		{
			name:       "arithmetic",
			expression: "1 + 2 * 3",
			want:       []tokenKind{tokenNumber, tokenPlus, tokenNumber, tokenStar, tokenNumber, tokenEOF},
		},
		{
			name:       "a function call",
			expression: "sqrt(16)",
			want:       []tokenKind{tokenIdent, tokenLParen, tokenNumber, tokenRParen, tokenEOF},
		},
		{
			name:       "postfix percent",
			expression: "50%",
			want:       []tokenKind{tokenNumber, tokenPercent, tokenEOF},
		},
		{
			name:       "whitespace is insignificant",
			expression: "  7\t-\n1 ",
			want:       []tokenKind{tokenNumber, tokenMinus, tokenNumber, tokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tokens, err := tokenize(tt.expression)
			if err != nil {
				t.Fatalf("tokenize(%q) error = %v", tt.expression, err)
			}
			got := kinds(tokens)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q) = %v, want %v", tt.expression, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("tokenize(%q)[%d] = %v, want %v", tt.expression, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// The UI renders these glyphs on its keys, so an expression pasted out of the
// frontend must lex without translation.
func TestTokenizeUnicodeAliases(t *testing.T) {
	t.Parallel()

	tokens, err := tokenize("2 × 3 ÷ 4 − 1")
	if err != nil {
		t.Fatalf("tokenize error = %v", err)
	}
	want := []tokenKind{tokenNumber, tokenStar, tokenNumber, tokenSlash, tokenNumber, tokenMinus, tokenNumber, tokenEOF}
	for i, kind := range kinds(tokens) {
		if kind != want[i] {
			t.Fatalf("token %d = %v, want %v", i, kind, want[i])
		}
	}

	radical, err := tokenize("√9")
	if err != nil {
		t.Fatalf("tokenize error = %v", err)
	}
	if radical[0].kind != tokenRadical {
		t.Errorf("√ lexed as %v, want the radical operator", radical[0].kind)
	}
}

func TestTokenizeNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expression string
		want       float64
	}{
		{"42", 42},
		{"3.5", 3.5},
		{".5", 0.5},
		{"2e5", 200000},
		{"2E5", 200000},
		{"2e-3", 0.002},
		{"2e+3", 2000},
	}

	for _, tt := range tests {
		tokens, err := tokenize(tt.expression)
		if err != nil {
			t.Fatalf("tokenize(%q) error = %v", tt.expression, err)
		}
		if tokens[0].kind != tokenNumber || tokens[0].value != tt.want {
			t.Errorf("tokenize(%q) = %v, want the number %v", tt.expression, tokens[0].value, tt.want)
		}
	}
}

// `e` is both an exponent marker and a constant. The exponent is only consumed
// when digits follow, so `2*e` keeps its constant.
func TestTokenizeExponentVersusEulerConstant(t *testing.T) {
	t.Parallel()

	tokens, err := tokenize("2*e")
	if err != nil {
		t.Fatalf("tokenize error = %v", err)
	}
	want := []tokenKind{tokenNumber, tokenStar, tokenIdent, tokenEOF}
	for i, kind := range kinds(tokens) {
		if kind != want[i] {
			t.Fatalf("token %d = %v, want %v", i, kind, want[i])
		}
	}
}

func TestTokenizeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		wantCode   string
		wantPos    int
	}{
		{"unknown character", "2 @ 3", CodeSyntax, 2},
		{"two decimal points", "1.2.3", CodeSyntax, 0},
		{"literal out of range", "1e400", CodeOverflow, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tokenize(tt.expression)
			if err == nil {
				t.Fatalf("tokenize(%q) error = nil, want %s", tt.expression, tt.wantCode)
			}
			if err.Code != tt.wantCode {
				t.Errorf("code = %s, want %s", err.Code, tt.wantCode)
			}
			if err.Position != tt.wantPos {
				t.Errorf("position = %d, want %d", err.Position, tt.wantPos)
			}
		})
	}
}

// Positions are rune offsets, so a multi-byte glyph earlier in the expression
// must not shift the reported column.
func TestTokenizePositionsAreRuneOffsets(t *testing.T) {
	t.Parallel()

	tokens, err := tokenize("√9 + 1")
	if err != nil {
		t.Fatalf("tokenize error = %v", err)
	}
	if tokens[2].pos != 3 {
		t.Errorf("plus at %d, want rune offset 3", tokens[2].pos)
	}
}
