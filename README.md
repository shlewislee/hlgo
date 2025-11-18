# hlgo

![GitHub License](https://img.shields.io/github/license/shlewislee/hlgo) ![GitHub go.mod Go version (branch)](https://img.shields.io/github/go-mod/go-version/shlewislee/hlgo/main) ![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/shlewislee/hlgo/go-test.yaml)


A Go binding for the `hledger` CLI accounting tool.

## Installation

```
$ go get github.com/shlewislee/hlgo
```

`hledger` installation is also required. By default, `hlgo` will look in `PATH` and then the user cache directory(`.cache`).

You can also explicitly provide `hledger` path.

### Installing hledger

You can also use the built-in `Install` function to download and install the `hledger` binary into the user cache directory:

```go
binPath, err := hlgo.Install(context.Background(), nil)
if err != nil {
  log.Fatal(err)
}
fmt.Println(binPath)
```

You can either provide the downloaded path to `New()` or just let `hlgo` look for the `.cache` directory.

Note that `hledger` installation via `hlgo` is only for x86 Linux. The installer does not check the OS and will proceed without error even if the OS differs. 

## Usage

```go
ctx := context.Background()

hl, err := hlgo.New(&hlgo.NewHledgerOption{
  LedgerPaths: []string{"examples/example.journal"},
})
if err != nil {
  log.Panic(err)
}

v, _ := hl.Version(ctx)
fmt.Println(v)

cmd := hl.NewCommand(
  "bal",
  hlgo.WithAccount("bank"),
  hlgo.WithPeriod(hlgo.PeriodDaily),
  hlgo.WithValuation("KRW", hlgo.ValuationThen),
  hlgo.WithInferMarketPrice(),
  hlgo.WithDate("2025-01-01.."),
  hlgo.WithHistorical(),
)

res, _ := cmd.Run(ctx) // []byte

fmt.Println(string(res))
```

There are also a few examples in the `examples/` directory.

### Streaming Output

For large reports (e.g. HTML, JSON output), you can also use `Command.Stream` to write output directly to an `io.Writer`(e.g. `os.Stdout`).

```go
cmdStream := hl.NewCommand("bal")
err = cmdStream.Stream(ctx, os.Stdout)
if err != nil {
    log.Fatal(err)
}
```

## Authors

- [@shlewislee](https://github.com/shlewislee)

## License

This project is licensed under the Mozilla Public License Version 2.0 (MPL-2.0).

For the full license text, please see the [LICENSE](LICENSE) file.
