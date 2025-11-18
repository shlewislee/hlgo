package hlgo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/mod/semver"
)

type Hledger struct {
	Binary       string
	JournalPaths []string
	DefaultArgs  []Option
}

type NewHledgerOption struct {
	BinaryPath  string
	LedgerPaths []string
	DefaultArgs []Option
}

// Version returns version string generated with
//
//	$ hledger --version
//
// . (hledger v0.00.0, linux-x86_64)
func (hl *Hledger) Version(ctx context.Context) (string, error) {
	verCmd, err := hl.NewCommand("",
		WithArg("--version")).Run(ctx)
	if err != nil {
		slog.Error("Error!", "stdout", err.(*CommandError).Stderr)
		return "", err
	}
	return string(verCmd), nil
}

func (opts *NewHledgerOption) fillDefaults() error {
	if opts.BinaryPath == "" {
		path, ok := checkIfInstalled()
		if !ok {
			return fmt.Errorf("No hledger installation found. Install hledger first.")
		}
		opts.BinaryPath = path
	}
	return nil
}

func checkIfInstalled() (string, bool) {
	pathInstall, err := exec.LookPath("hledger")
	if err == nil {
		return pathInstall, true
	}
	if cached, err := latestCachedBinary(); err != nil {
		return "", false
	} else {
		return cached, true
	}
}

func IsInstalled() (string, bool) {
	return checkIfInstalled()
}

func latestCachedBinary() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(filepath.Join(cacheDir, "hlgo/bin"))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("No cached binary found. Install hledger first.")
	}

	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		versions = append(versions, e.Name())
	}
	semver.Sort(versions) // Use latest binary
	latestBin := filepath.Join(cacheDir, "hlgo/bin", versions[len(versions)-1], "hledger")
	if _, err := os.Stat(latestBin); err != nil {
		return "", fmt.Errorf("Empty latest binary .cache dir: %w", err)
	}

	return latestBin, nil
}

// [New] starts a [Hledger] instance.
//
// Requires hledger installation. New will check the user cache directory if hledger is not available on PATH or if no explicit binary path is given.
//
// If multiple hledger versions are found in the cache directory, New will use the latest version determined with [semver]
func New(opts *NewHledgerOption) (*Hledger, error) {
	if opts == nil {
		opts = &NewHledgerOption{}
	}
	if err := opts.fillDefaults(); err != nil {
		return nil, err
	}
	return &Hledger{
		Binary:       opts.BinaryPath,
		JournalPaths: opts.LedgerPaths,
		DefaultArgs:  opts.DefaultArgs,
	}, nil
}
