// scenario-contract is the shared validation boundary for generator and runner tooling.
package main

import (
	"encoding/json"
	"github.com/compfly-ai/crosswind/api/pkg/scenario"
	"io"
	"os"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "schema" {
		os.Stdout.Write(scenario.Schema)
		return
	}
	b, e := io.ReadAll(io.LimitReader(os.Stdin, scenario.MaxDocumentBytes+1))
	if e != nil {
		panic(e)
	}
	r := scenario.Validate(b)
	json.NewEncoder(os.Stdout).Encode(r)
	if !r.Valid {
		os.Exit(1)
	}
}
