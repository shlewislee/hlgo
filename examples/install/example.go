package main

import (
	"context"
	"fmt"
	"log"

	"github.com/shlewislee/hlgo"
)

func main() {
	ctx := context.Background()
	binPath, err := hlgo.Install(ctx, &hlgo.InstallHledgerOption{
		Version: "latest",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(binPath)

	hlCached, err := hlgo.New(&hlgo.NewHledgerOption{
		LedgerPaths: []string{"examples/example.journal"},
		BinaryPath:  binPath,
	})
	if err != nil {
		log.Fatal(err)
	}

	v, err := hlCached.NewCommand("bal").Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(v))
}
