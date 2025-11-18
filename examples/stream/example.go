package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/shlewislee/hlgo"
)

func main() {
	ctx := context.Background()

	hl, err := hlgo.New(&hlgo.NewHledgerOption{
		LedgerPaths: []string{"examples/example.journal"},
	})
	if err != nil {
		log.Panic(err)
	}

	// Writes the result directly to an io.Writer (like os.Stdout).
	// Best for large reports (like CSV) to avoid using memory.
	fmt.Println("--- Running with Stream() ---")
	cmdStream := hl.NewCommand(
		"bal",
	)

	// We stream directly to the terminal's standard output.
	// No data is captured into a byte slice.
	err = cmdStream.Stream(ctx, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("-----------------------------")
}
