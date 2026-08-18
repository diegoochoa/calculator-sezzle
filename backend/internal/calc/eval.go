package calc

import "fmt"

// Evaluate walks a parsed expression. Every arithmetic result is guarded, so a
// NaN or ±Inf becomes a typed error here rather than an unencodable JSON body
// at the transport layer.
func Evaluate(node Node) (float64, *Error) {
	value, err := eval(node)
	if err != nil {
		return 0, err
	}
	guarded, err := guard(value)
	if err != nil {
		return 0, err.at(node.Pos())
	}
	return guarded, nil
}

func eval(node Node) (float64, *Error) {
	switch n := node.(type) {
	case *NumberNode:
		return n.Value, nil

	case *UnaryNode:
		return evalUnary(n)

	case *PostfixNode:
		return evalPostfix(n)

	case *BinaryNode:
		return evalBinary(n)

	case *CallNode:
		return evalCall(n)

	default:
		return 0, newError(CodeSyntax, fmt.Sprintf("Unsupported expression element %T", node), node.Pos())
	}
}

func evalUnary(n *UnaryNode) (float64, *Error) {
	operand, err := eval(n.X)
	if err != nil {
		return 0, err
	}

	switch n.Op {
	case "+":
		return operand, nil
	case "-":
		return -operand, nil
	case "√":
		result, applyErr := functions["sqrt"].apply([]float64{operand})
		return result, applyErr.at(n.Offset)
	default:
		return 0, newError(CodeSyntax, "Unsupported prefix operator "+n.Op, n.Offset)
	}
}

func evalPostfix(n *PostfixNode) (float64, *Error) {
	operand, err := eval(n.X)
	if err != nil {
		return 0, err
	}

	switch n.Op {
	case "%":
		// Postfix percent is plain division by 100.
		result, opErr := Div(operand, 100)
		return result, opErr.at(n.Offset)
	default:
		return 0, newError(CodeSyntax, "Unsupported postfix operator "+n.Op, n.Offset)
	}
}

func evalBinary(n *BinaryNode) (float64, *Error) {
	left, err := eval(n.Left)
	if err != nil {
		return 0, err
	}
	right, err := eval(n.Right)
	if err != nil {
		return 0, err
	}

	var (
		result float64
		opErr  *Error
	)
	switch n.Op {
	case "+":
		result = Add(left, right)
	case "-":
		result = Sub(left, right)
	case "*":
		result = Mul(left, right)
	case "/":
		result, opErr = Div(left, right)
	case "^":
		result, opErr = Pow(left, right)
	default:
		return 0, newError(CodeSyntax, "Unsupported operator "+n.Op, n.Offset)
	}
	if opErr != nil {
		return 0, opErr.at(n.Offset)
	}

	guarded, guardErr := guard(result)
	if guardErr != nil {
		return 0, guardErr.at(n.Offset)
	}
	return guarded, nil
}

func evalCall(n *CallNode) (float64, *Error) {
	def, ok := functions[n.Name]
	if !ok {
		// Unreachable: the parser rejects unknown functions.
		return 0, newError(CodeUnknownIdentifier, "Unknown function "+n.Name, n.Offset)
	}

	args := make([]float64, len(n.Args))
	for i, arg := range n.Args {
		value, err := eval(arg)
		if err != nil {
			return 0, err
		}
		args[i] = value
	}

	result, err := def.apply(args)
	if err != nil {
		return 0, err.at(n.Offset)
	}

	guarded, guardErr := guard(result)
	if guardErr != nil {
		return 0, guardErr.at(n.Offset)
	}
	return guarded, nil
}
