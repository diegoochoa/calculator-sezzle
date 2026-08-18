package calc

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Precedence climbing parser.
//
//	expression → binary(1)
//	binary(n)  → unary { infixOp binary(n') }
//	unary      → ("-" | "+" | "√") unary | power
//	power      → postfix [ "^" unary ]        -- right associative
//	postfix    → primary { "!" | "%" }
//	primary    → number | constant | call | "(" expression ")"
//
// Unary minus binds looser than "^", so -2^2 is -4, and the exponent is parsed
// as a unary so 2^-3 works. Implicit multiplication is deliberately absent:
// "2pi" is a syntax error rather than a guess.

type opInfo struct {
	name       string
	precedence int
	rightAssoc bool
}

type parser struct {
	tokens   []token
	index    int
	depth    int
	maxDepth int
}

// Parse turns an expression into an AST, validating syntax, arity, identifiers
// and structural limits. It never evaluates, so it is also the implementation
// behind the validate endpoint.
func Parse(expression string, maxLength, maxDepth int) (Node, *Error) {
	if strings.TrimSpace(expression) == "" {
		return nil, newError(CodeEmpty, "The expression is empty", NoPosition)
	}
	if length := utf8.RuneCountInString(expression); maxLength > 0 && length > maxLength {
		return nil, newError(CodeTooLong,
			fmt.Sprintf("The expression is %d characters, the limit is %d", length, maxLength), NoPosition)
	}

	tokens, err := tokenize(expression)
	if err != nil {
		return nil, err
	}

	if maxDepth <= 0 {
		maxDepth = 32
	}
	p := &parser{tokens: tokens, maxDepth: maxDepth}

	node, err := p.parseBinary(1)
	if err != nil {
		return nil, err
	}
	if current := p.current(); current.kind != tokenEOF {
		return nil, newError(CodeSyntax, "Unexpected "+current.kind.String(), current.pos)
	}
	return node, nil
}

func (p *parser) current() token { return p.tokens[p.index] }

func (p *parser) advance() token {
	tok := p.tokens[p.index]
	if tok.kind != tokenEOF {
		p.index++
	}
	return tok
}

// enter bounds recursion. Without it a hostile input such as 5000 nested
// parentheses overflows the goroutine stack, and a stack overflow in Go is
// unrecoverable — it takes the whole process down, not just the request.
func (p *parser) enter() *Error {
	p.depth++
	if p.depth > p.maxDepth {
		return newError(CodeDepthExceeded,
			fmt.Sprintf("The expression nests deeper than %d levels", p.maxDepth), p.current().pos)
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

// peekInfix reports the infix operator at the cursor, if any.
func (p *parser) peekInfix() (opInfo, bool) {
	switch tok := p.current(); tok.kind {
	case tokenPlus:
		return opInfo{name: "+", precedence: 1}, true
	case tokenMinus:
		return opInfo{name: "-", precedence: 1}, true
	case tokenStar:
		return opInfo{name: "*", precedence: 2}, true
	case tokenSlash:
		return opInfo{name: "/", precedence: 2}, true
	}
	return opInfo{}, false
}

func (p *parser) parseBinary(minPrecedence int) (Node, *Error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		op, ok := p.peekInfix()
		if !ok || op.precedence < minPrecedence {
			return left, nil
		}

		operator := p.advance()
		next := op.precedence + 1
		if op.rightAssoc {
			next = op.precedence
		}

		right, err := p.parseBinary(next)
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: op.name, Left: left, Right: right, Offset: operator.pos}
	}
}

func (p *parser) parseUnary() (Node, *Error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	switch tok := p.current(); tok.kind {
	case tokenPlus, tokenMinus, tokenRadical:
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		op := map[tokenKind]string{tokenPlus: "+", tokenMinus: "-", tokenRadical: "√"}[tok.kind]
		return &UnaryNode{Op: op, X: operand, Offset: tok.pos}, nil
	}

	return p.parsePower()
}

func (p *parser) parsePower() (Node, *Error) {
	base, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}

	if p.current().kind != tokenCaret {
		return base, nil
	}

	operator := p.advance()
	// Parsing the exponent as a unary makes "^" right associative and lets
	// 2^-3 work without parentheses.
	exponent, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	return &BinaryNode{Op: "^", Left: base, Right: exponent, Offset: operator.pos}, nil
}

func (p *parser) parsePostfix() (Node, *Error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.current()
		if tok.kind != tokenPercent {
			return node, nil
		}
		p.advance()
		node = &PostfixNode{Op: "%", X: node, Offset: tok.pos}
	}
}

func (p *parser) parsePrimary() (Node, *Error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	switch tok := p.current(); tok.kind {
	case tokenNumber:
		p.advance()
		return &NumberNode{Value: tok.value, Offset: tok.pos}, nil

	case tokenLParen:
		p.advance()
		inner, err := p.parseBinary(1)
		if err != nil {
			return nil, err
		}
		if p.current().kind != tokenRParen {
			return nil, newError(CodeUnbalancedParen, "Missing a closing parenthesis", p.current().pos)
		}
		p.advance()
		return inner, nil

	case tokenRParen:
		return nil, newError(CodeUnbalancedParen, "Unexpected closing parenthesis", tok.pos)

	case tokenIdent:
		return p.parseIdent()

	case tokenEOF:
		return nil, newError(CodeSyntax, "The expression ends unexpectedly", tok.pos)

	default:
		return nil, newError(CodeSyntax, "Expected a number or name but found "+tok.kind.String(), tok.pos)
	}
}

func (p *parser) parseIdent() (Node, *Error) {
	name := p.advance()

	if p.current().kind == tokenLParen {
		return p.parseCall(name)
	}

	if _, ok := functions[name.text]; ok {
		return nil, newError(CodeSyntax,
			fmt.Sprintf("%s is a function, it needs parentheses such as %s(1)", name.text, name.text), name.pos)
	}
	return nil, newError(CodeUnknownIdentifier, "Unknown name "+strconv.Quote(name.text), name.pos)
}

func (p *parser) parseCall(name token) (Node, *Error) {
	def, ok := functions[name.text]
	if !ok {
		return nil, newError(CodeUnknownIdentifier, "Unknown function "+strconv.Quote(name.text), name.pos)
	}

	p.advance() // consume "("

	var args []Node
	if p.current().kind != tokenRParen {
		for {
			arg, err := p.parseBinary(1)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)

			if p.current().kind != tokenComma {
				break
			}
			p.advance()
		}
	}

	if p.current().kind != tokenRParen {
		return nil, newError(CodeUnbalancedParen,
			"Missing a closing parenthesis for "+name.text, p.current().pos)
	}
	p.advance()

	if len(args) < def.MinArgs || len(args) > def.MaxArgs {
		return nil, newError(CodeWrongArity, arityMessage(def, len(args)), name.pos)
	}
	return &CallNode{Name: name.text, Args: args, Offset: name.pos}, nil
}
