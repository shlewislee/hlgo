package hlgo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type ghReleaseJSON struct {
	TagName     string        `json:"tag_name"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []ghAssetJson `json:"assets"`
}

type ghAssetJson struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type hlDownloadInfo struct {
	TagName string
	URL     string
}

// if Logger is not nil, LogLevel will be ignored.
type InstallHledgerOption struct {
	Dir     string
	Version string

	Logger   *slog.Logger
	LogLevel slog.Level
}

func (opts *InstallHledgerOption) fillDefaults() error {
	if opts.Version == "" {
		opts.Version = "latest"
	}
	if opts.Dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf(".cache dir is inaccesible. Please manually set installation path for installation: %w", err)
		}
		opts.Dir = filepath.Join(cacheDir, "hlgo")
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: opts.LogLevel, // Defaults to zero
		}))
	}
	return nil
}

// Install downloads the hledger binary to the given path.
//
// Use this before [New] or manually install hledger.
func Install(ctx context.Context, opts *InstallHledgerOption) (string, error) {
	if opts == nil {
		opts = &InstallHledgerOption{}
	}
	if err := opts.fillDefaults(); err != nil {
		return "", err
	}

	opts.Logger.Info("Checking hledger installation...", "version", opts.Version)

	dlInfo, err := getHlDownloadInfo(ctx, opts.Version)
	if err != nil {
		return "", err
	}

	cleanTagName := filepath.Base(dlInfo.TagName)
	if cleanTagName == "." || cleanTagName == ".." || strings.Contains(cleanTagName, string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid tag name received from GitHub API: %q", dlInfo.TagName)
	}

	targetDir := filepath.Join(opts.Dir, "bin", "v"+cleanTagName)
	targetFile := filepath.Join(targetDir, "hledger")

	installed, err := checkExistingInstall(targetFile, dlInfo.TagName, opts.Logger)
	if err != nil {
		return "", err
	}
	if installed {
		return targetFile, nil
	}

	return downloadAndExtract(ctx, dlInfo, targetFile, targetDir, opts.Logger)
}

func checkExistingInstall(targetFile, tagName string, logger *slog.Logger) (bool, error) {
	if _, err := os.Stat(targetFile); err == nil {
		logger.Info("hledger already installed", "version", tagName, "path", targetFile)
		if err := os.Chmod(targetFile, 0755); err != nil {
			logger.Warn("Failed to set executable permission on existing file", "file", targetFile, "error", err)
		}
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check destination file status: %w", err)
	}
	return false, nil
}

func downloadAndExtract(ctx context.Context, dlInfo *hlDownloadInfo, targetFile, targetDir string, logger *slog.Logger) (string, error) {
	logger.Info("Downloading hledger tarball...", "version", dlInfo.TagName, "from", dlInfo.URL)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, dlInfo.URL, nil)
	req.Header.Add("Accept", "application/octet-stream")

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)

	if err != nil {
		return "", fmt.Errorf("error while requesting hledger asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download asset: %s", resp.Status)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	if err := extractBinaryFromTar(tr, targetFile, logger); err != nil {
		return "", err
	}

	logger.Info("Download and extraction completed successfully", "path", targetFile)
	return targetFile, nil
}

func extractBinaryFromTar(tr *tar.Reader, targetFile string, logger *slog.Logger) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		isHlegerBin := hdr.Typeflag == tar.TypeReg && hdr.Name == "hledger"

		if isHlegerBin {
			logger.Info("Extracting binary...", "file", hdr.Name, "to", targetFile)

			outFile, err := os.Create(targetFile)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}

			_, copyErr := io.Copy(outFile, tr)
			closeErr := outFile.Close()

			if copyErr != nil {
				os.Remove(targetFile)
				return fmt.Errorf("failed to save asset: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("failed to close destination file: %w", closeErr)
			}

			if err := os.Chmod(targetFile, 0755); err != nil {
				return fmt.Errorf("failed to set executable permission: %w", err)
			}

			return nil // extracted
		}
	}
	return fmt.Errorf("failed to find 'hledger' binary in the downloaded archive")
}

func getHlDownloadInfo(ctx context.Context, version string) (*hlDownloadInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildHlReleaseUrl(version), nil)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch release info: %s", resp.Status)
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonRes *ghReleaseJSON
	if err := json.Unmarshal(resBody, &jsonRes); err != nil {
		return nil, err
	}

	const targetAsset = "hledger-linux-x64.tar.gz"
	linuxIdx := slices.IndexFunc(jsonRes.Assets, func(a ghAssetJson) bool {
		return a.Name == targetAsset
	})

	if linuxIdx == -1 {
		var assetNames []string
		for _, a := range jsonRes.Assets {
			assetNames = append(assetNames, a.Name)
		}
		slog.Warn("Target asset not found", "target", targetAsset, "available_assets", assetNames)
		return nil, fmt.Errorf("invalid assets result: '%s' not found", targetAsset)
	}

	return &hlDownloadInfo{TagName: jsonRes.TagName, URL: jsonRes.Assets[linuxIdx].URL}, nil
}

func buildHlReleaseUrl(version string) string {
	var reqUrlSb strings.Builder
	reqUrlSb.WriteString("https://api.github.com/repos/simonmichael/hledger/releases")
	if version != "latest" {
		reqUrlSb.WriteString("/tags/")
		reqUrlSb.WriteString(version)
	} else {
		reqUrlSb.WriteString("/latest")
	}
	return reqUrlSb.String()
}
