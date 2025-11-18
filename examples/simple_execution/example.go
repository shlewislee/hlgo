package main

import (
	"context"
	"fmt"
	"log"

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

	v, _ := hl.Version(ctx)

	fmt.Printf("Using %s (%s)\n", hl.Binary, v)

	cmd := hl.NewCommand(
		"bal",
		hlgo.WithAccount("bank"),
		hlgo.WithPeriod(hlgo.PeriodDaily),
		hlgo.WithValuation("KRW", hlgo.ValuationThen),
		hlgo.WithInferMarketPrice(),
		hlgo.WithDate("2025-01-01.."),
		hlgo.WithHistorical(),
	)

	fmt.Printf("Command: %s\n", cmd.String())

	res, _ := cmd.Run(ctx)

	fmt.Println(string(res))
}
