package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/shlewislee/hlgo"
)

func main() {
	hl, err := hlgo.New(&hlgo.NewHledgerOption{
		LedgerPaths: []string{"examples/invalid.journal"},
	})
	if err != nil {
		log.Panic(err)
	}

	cmd := hl.NewCommand(
		"bal",
		hlgo.WithAccount("bank"),
		hlgo.WithHistorical(),
		hlgo.WithPeriod(hlgo.PeriodDaily),
		hlgo.WithQuery(hlgo.QueryTypeAmt, ">100"),
	)

	res, err := cmd.Run(context.Background())
	if err != nil {
		slog.Error("Command failed", "stderr", err.(*hlgo.CommandError).Stderr)

	}

	fmt.Println(string(res))
}
