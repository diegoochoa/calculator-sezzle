package calc

// Node is one element of the parsed expression tree. Pos is the rune offset the
// node starts at, carried so runtime failures can be reported against the input.
type Node interface {
	Pos() int
}

// NumberNode is a literal.
type NumberNode struct {
	Value  float64
	Offset int
}

func (n *NumberNode) Pos() int { return n.Offset }

// UnaryNode is a prefix operator: -x, +x, √x.
type UnaryNode struct {
	Op     string
	X      Node
	Offset int
}

func (n *UnaryNode) Pos() int { return n.Offset }

// PostfixNode is a suffix operator: x!, x%.
type PostfixNode struct {
	Op     string
	X      Node
	Offset int
}

func (n *PostfixNode) Pos() int { return n.Offset }

// BinaryNode is an infix operator. Offset points at the operator itself, which
// is where a division by zero is most usefully underlined.
type BinaryNode struct {
	Op     string
	Left   Node
	Right  Node
	Offset int
}

func (n *BinaryNode) Pos() int { return n.Offset }

// CallNode is a function application such as sin(30) or log(8, 2).
type CallNode struct {
	Name   string
	Args   []Node
	Offset int
}

func (n *CallNode) Pos() int { return n.Offset }
