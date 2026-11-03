// scenario-contract is the shared validation boundary for generator and runner tooling.
package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/compfly-ai/crosswind/api/pkg/scenario"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "schema" {
		if _, err := os.Stdout.Write(scenario.Schema); err != nil {
			panic(err)
		}
		return
	}
	b, e := io.ReadAll(io.LimitReader(os.Stdin, scenario.MaxDocumentBytes+1))
	if e != nil {
		panic(e)
	}
	r := scenario.Validate(b)
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		panic(err)
	}
	if !r.Valid {
		os.Exit(1)
	}
}
