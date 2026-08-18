package calc

// The catalog describes what the engine understands. It is served so a client
// can build its keypad and its syntax hints from the server's real capability
// rather than a hand-maintained duplicate.

// FunctionInfo documents one callable function.
type FunctionInfo struct {
	Name    string `json:"name"`
	MinArgs int    `json:"minArgs"`
	MaxArgs int    `json:"maxArgs"`
	Summary string `json:"summary"`
	Domain  string `json:"domain,omitempty"`
}

// OperatorInfo documents one operator.
type OperatorInfo struct {
	Symbol        string   `json:"symbol"`
	Summary       string   `json:"summary"`
	Fixity        string   `json:"fixity"`
	Precedence    int      `json:"precedence"`
	Associativity string   `json:"associativity,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
}

// Catalog is the full description of the grammar.
type Catalog struct {
	Functions []FunctionInfo `json:"functions"`
	Operators []OperatorInfo `json:"operators"`
	MaxLength int            `json:"maxExpressionLength"`
	MaxDepth  int            `json:"maxDepth"`
}

var operatorCatalog = []OperatorInfo{
	{Symbol: "+", Summary: "Addition", Fixity: "infix", Precedence: 1, Associativity: "left"},
	{Symbol: "-", Summary: "Subtraction", Fixity: "infix", Precedence: 1, Associativity: "left", Aliases: []string{"−"}},
	{Symbol: "*", Summary: "Multiplication", Fixity: "infix", Precedence: 2, Associativity: "left", Aliases: []string{"×", "·"}},
	{Symbol: "/", Summary: "Division", Fixity: "infix", Precedence: 2, Associativity: "left", Aliases: []string{"÷"}},
	{Symbol: "-", Summary: "Negation", Fixity: "prefix", Precedence: 3},
	{Symbol: "√", Summary: "Square root", Fixity: "prefix", Precedence: 3},
	{Symbol: "^", Summary: "Exponentiation", Fixity: "infix", Precedence: 4, Associativity: "right"},
	{Symbol: "%", Summary: "Divide by one hundred", Fixity: "postfix", Precedence: 5},
}

// NewCatalog builds the catalog for the given limits.
func NewCatalog(opts Options) Catalog {
	o := opts.withDefaults()

	catalog := Catalog{
		Functions: make([]FunctionInfo, 0, len(functions)),
		Operators: operatorCatalog,
		MaxLength: o.MaxLength,
		MaxDepth:  o.MaxDepth,
	}

	// Sorted names keep the payload stable, which makes it cacheable and makes
	// golden-file tests meaningful.
	for _, name := range FunctionNames() {
		def := functions[name]
		catalog.Functions = append(catalog.Functions, FunctionInfo{
			Name:    def.Name,
			MinArgs: def.MinArgs,
			MaxArgs: def.MaxArgs,
			Summary: def.Summary,
			Domain:  def.Domain,
		})
	}

	return catalog
}
