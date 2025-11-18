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
		DefaultArgs: []hlgo.Option{
			hlgo.WithDate("2025-01-01.."),
		},
	})
	if err != nil {
		log.Panic(err)
	}

	res1, _ := hl.NewCommand(
		"bal",
		hlgo.WithAccount("bank"),
		hlgo.WithHistorical(),
		hlgo.WithPeriod(hlgo.PeriodDaily),
		hlgo.WithQuery(hlgo.QueryTypeAmt, ">100"),
	).Run(ctx)

	res2, _ := hl.NewCommand(
		"print",
	).Run(ctx)

	fmt.Println(string(res1))
	fmt.Println(string(res2))
}
