package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMobileTemplateDeniesDesktopTools(t *testing.T) {
	if !strings.Contains(defaultYAML, mobilePermissionsFrom) {
		t.Fatal("default template changed: update mobilePermissionsFrom")
	}
	t.Setenv("GREGAL_PLATFORM", "android")
	t.Setenv("GREGAL_AUTH", filepath.Join(t.TempDir(), "auth.yaml"))
	c, _, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("mobile config invalid: %v", err)
	}
	for _, name := range []string{"bash", "bash_background", "browser", "git_commit", "gh_pr_create"} {
		if c.Permissions.Tools[name] != "deny" {
			t.Fatalf("%s = %q on mobile", name, c.Permissions.Tools[name])
		}
	}
	if c.Permissions.Tools["write"] != "ask" {
		t.Fatalf("write = %q", c.Permissions.Tools["write"])
	}
}

func TestDesktopTemplateUnchanged(t *testing.T) {
	t.Setenv("GREGAL_PLATFORM", "")
	if MobileDevice() {
		t.Skip("running on a mobile device")
	}
	if defaultTemplate() != defaultYAML {
		t.Fatal("desktop template must stay the default one")
	}
}
