package calc

import (
	"encoding/json"
	"testing"
)

func TestNewCatalogCoversTheGrammar(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog(Options{})

	if len(catalog.Functions) != len(functions) {
		t.Errorf("catalog lists %d functions, the engine has %d", len(catalog.Functions), len(functions))
	}
	if catalog.MaxLength != DefaultMaxLength || catalog.MaxDepth != DefaultMaxDepth {
		t.Errorf("catalog limits = (%d, %d), want the defaults", catalog.MaxLength, catalog.MaxDepth)
	}
}

// The catalog is a contract for clients building a keypad from it, so every
// advertised function must actually be callable with its stated arity.
func TestCatalogFunctionsAreCallable(t *testing.T) {
	t.Parallel()

	for _, info := range NewCatalog(Options{}).Functions {
		def, ok := functions[info.Name]
		if !ok {
			t.Errorf("catalog advertises %q, which the engine does not implement", info.Name)
			continue
		}
		if def.apply == nil {
			t.Errorf("%q has no implementation", info.Name)
		}
		if info.MinArgs > info.MaxArgs {
			t.Errorf("%q advertises minArgs %d > maxArgs %d", info.Name, info.MinArgs, info.MaxArgs)
		}
		if info.Summary == "" {
			t.Errorf("%q has no summary, so a client cannot label its key", info.Name)
		}
	}
}

func TestCatalogIsStableAndSerialisable(t *testing.T) {
	t.Parallel()

	first, err := json.Marshal(NewCatalog(Options{}))
	if err != nil {
		t.Fatalf("marshalling the catalog: %v", err)
	}
	second, err := json.Marshal(NewCatalog(Options{}))
	if err != nil {
		t.Fatalf("marshalling the catalog: %v", err)
	}

	// Map iteration order must not leak into the payload, or the response stops
	// being cacheable and golden tests become flaky.
	if string(first) != string(second) {
		t.Error("the catalog payload is not stable between calls")
	}
}

func TestCatalogRespectsOptions(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog(Options{MaxLength: 64, MaxDepth: 8})
	if catalog.MaxLength != 64 || catalog.MaxDepth != 8 {
		t.Errorf("catalog limits = (%d, %d), want (64, 8)", catalog.MaxLength, catalog.MaxDepth)
	}
}
