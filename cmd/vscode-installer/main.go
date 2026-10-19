package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/alsolanes/gregal/releases/latest"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	Assets []releaseAsset `json:"assets"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Gregal installer:", err)
		fmt.Fprintln(os.Stderr, "Press Enter to close.")
		_, _ = fmt.Scanln()
		os.Exit(1)
	}
}

func run() error {
	if runtime.GOOS != "windows" {
		return errors.New("this installer is for Windows")
	}
	codePath, err := findVSCode()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Minute}
	release, err := latestRelease(ctx, client)
	if err != nil {
		return err
	}
	vsix, backend, err := releasePackages(release.Assets)
	if err != nil {
		return err
	}

	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return errors.New("LOCALAPPDATA is not set")
	}
	installDir := filepath.Join(base, "Gregal")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install folder: %w", err)
	}
	vsixPath := filepath.Join(installDir, vsix.Name)
	backendPath := filepath.Join(installDir, "gregal.exe")
	if err := download(ctx, client, vsix.BrowserDownloadURL, vsixPath); err != nil {
		return fmt.Errorf("download VS Code extension: %w", err)
	}
	if err := download(ctx, client, backend.BrowserDownloadURL, backendPath); err != nil {
		return fmt.Errorf("download Gregal backend: %w", err)
	}

	cmd := exec.CommandContext(ctx, codePath, "--install-extension", vsixPath, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install VS Code extension: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	fmt.Println("Gregal is installed. Keep this window open while using VS Code; close it or press Ctrl+C to stop the backend.")
	fmt.Println("Configure a model provider before sending your first request.")
	cancel()
	server := exec.Command(backendPath, "--serve")
	server.Stdin, server.Stdout, server.Stderr = os.Stdin, os.Stdout, os.Stderr
	return server.Run()
}

func latestRelease(ctx context.Context, client *http.Client) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gregal-vscode-installer")
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("get latest GitHub release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("get latest GitHub release: GitHub returned %s", resp.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("read latest GitHub release: %w", err)
	}
	return release, nil
}

func releasePackages(assets []releaseAsset) (releaseAsset, releaseAsset, error) {
	var vsix, backend releaseAsset
	for _, asset := range assets {
		if strings.HasPrefix(asset.Name, "gregal-") && strings.HasSuffix(asset.Name, ".vsix") {
			vsix = asset
		}
		if asset.Name == "gregal-windows-amd64.exe" {
			backend = asset
		}
	}
	if vsix.Name == "" || backend.Name == "" || vsix.BrowserDownloadURL == "" || backend.BrowserDownloadURL == "" {
		return releaseAsset{}, releaseAsset{}, errors.New("the latest release does not include both the VS Code extension and Windows backend")
	}
	return vsix, backend, nil
}

func download(ctx context.Context, client *http.Client, url, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	tmp := destination + ".download"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func findVSCode() (string, error) {
	candidates := make([]string, 0, 4)
	if command, err := exec.LookPath("code"); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(filepath.Dir(command)), "Code.exe"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "Programs", "Microsoft VS Code", "Code.exe"))
	}
	if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
		candidates = append(candidates, filepath.Join(programFiles, "Microsoft VS Code", "Code.exe"))
	}
	if programFiles := os.Getenv("ProgramFiles(x86)"); programFiles != "" {
		candidates = append(candidates, filepath.Join(programFiles, "Microsoft VS Code", "Code.exe"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("VS Code was not found; install it before running the Gregal installer")
}
