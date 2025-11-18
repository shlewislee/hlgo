package hlgo

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

const testJournalPath = "testdata/test.journal"
const testResPath = "testdata/result.json"

func setupClient(t *testing.T) *Hledger {
	t.Helper()
	hl, err := New(&NewHledgerOption{
		LedgerPaths: []string{testJournalPath},
	})
	if err != nil {
		t.Fatalf("Failed to create hledger client: %v", err)
	}
	return hl
}

func TestInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping installation test in short mode...")
	}

	ctx := context.Background()

	binPath, err := Install(ctx, &InstallHledgerOption{
		LogLevel: slog.LevelDebug,
	})

	if err != nil {
		t.Fatalf("Installation failed: %v", err)
	}

	if _, err := os.Stat(binPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Errorf("Installed binary not found in %s: %v", binPath, err)
		}
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestVersion(t *testing.T) {
	hl := setupClient(t)
	ctx := context.Background()

	v, err := hl.Version(ctx)
	if err != nil {
		if errors.Is(err, &CommandError{}) {
			t.Fatalf("Version command failed: %s\nstderr: %s\n", err, err.(*CommandError).Stderr)
		}
		t.Fatalf("Version command failed: %v", err)
	}
	t.Logf("hledger execution success: %s\nversion: %s", hl.Binary, v)
}

func TestComplexJsonOutput(t *testing.T) {
	hl := setupClient(t)
	ctx := context.Background()

	cmd := hl.NewCommand(
		"bal",
		WithHistorical(),
		WithPeriod(PeriodDaily),
		WithDate("2025-01-01..2025-03-01"),
		WithInferMarketPrice(),
		WithValuation("KRW", ValuationThen),
		WithOutputType(OutputJSON),
	)

	t.Logf("Executing following command: %s", cmd.String())

	resBytes, err := cmd.Run(ctx)
	if err != nil {
		if errors.Is(err, &CommandError{}) {
			t.Fatalf("Command execution failed: %s\nstderr: %s\n", err, err.(*CommandError).Stderr)
		}
		t.Fatalf("Command execution failed: %v", err)
	}

	result := string(resBytes)
	expectedBytes, err := os.ReadFile(testResPath)

	if err != nil {
		t.Fatalf("Failed to read test result file %s: %v", testResPath, err)
	}

	t.Logf("Comparing with the expected file: %s", testResPath)

	if !bytes.Equal(resBytes, expectedBytes) {
		t.Errorf("JSON output mismatch\n")
		t.Logf("Expected (from %s):\n%s", testResPath, expectedBytes)
		t.Logf("Output received:\n%s", result)
	}
}
