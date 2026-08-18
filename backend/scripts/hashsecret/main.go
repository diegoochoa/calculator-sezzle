// Command hashsecret bcrypts a client secret so it can go into CALC_CLIENTS.
//
//	go run ./scripts/hashsecret "my-secret"
//	CALC_CLIENTS=web:$2a$10$...
package main

import (
	"fmt"
	"os"

	"github.com/diegoochoa/calculator-sezzle-api/internal/config"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "" {
		fmt.Fprintln(os.Stderr, `usage: hashsecret "your-client-secret"`)
		os.Exit(1)
	}

	hash, err := config.HashSecret(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashing failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}
