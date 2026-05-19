package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/consensys/linea-monorepo/verifier-ray/codegen/internal/generator"
)

func main() {
	entryPoint := flag.String("entry", "verifyGenerated", "name of the generated Zig verifier entry point")
	flag.Parse()

	system := generator.System{
		Rounds: []generator.Round{
			{ID: 0, VerifierActions: []string{"stub"}},
		},
	}

	if err := generator.Generate(system, generator.Options{EntryPoint: *entryPoint}, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ray-zig-codegen: %v\n", err)
		os.Exit(1)
	}
}
