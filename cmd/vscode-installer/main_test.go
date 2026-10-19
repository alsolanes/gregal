package main

import "testing"

func TestReleasePackages(t *testing.T) {
	assets := []releaseAsset{
		{Name: "gregal-0.6.0.vsix", BrowserDownloadURL: "https://example.test/gregal.vsix"},
		{Name: "gregal-windows-amd64.exe", BrowserDownloadURL: "https://example.test/gregal.exe"},
		{Name: "gregal-linux-amd64", BrowserDownloadURL: "https://example.test/linux"},
	}
	vsix, backend, err := releasePackages(assets)
	if err != nil {
		t.Fatal(err)
	}
	if vsix.Name != "gregal-0.6.0.vsix" || backend.Name != "gregal-windows-amd64.exe" {
		t.Fatalf("selected %q and %q", vsix.Name, backend.Name)
	}
}

func TestReleasePackagesRequiresBothFiles(t *testing.T) {
	_, _, err := releasePackages([]releaseAsset{{Name: "gregal-0.6.0.vsix", BrowserDownloadURL: "https://example.test/gregal.vsix"}})
	if err == nil {
		t.Fatal("expected an error when the backend asset is missing")
	}
}
