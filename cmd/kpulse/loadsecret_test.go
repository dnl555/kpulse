package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadSecretDirHandlesK8sProjection mimics the directory structure
// Kubernetes creates when mounting a Secret: a "..data" symlink to a
// timestamped sub-directory, and the actual keys exposed as flat symlinks
// at the top level. Our loader must read the flat keys and ignore the
// bookkeeping entries.
func TestLoadSecretDirHandlesK8sProjection(t *testing.T) {
	dir := t.TempDir()
	// Timestamped data dir.
	dataDir := filepath.Join(dir, "..2026_05_24_09_06_08.123456")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "SLACK_WEBHOOK_URL"), []byte("https://hooks.slack.com/x"), 0644); err != nil {
		t.Fatal(err)
	}
	// The "..data" symlink that points to the timestamped dir.
	if err := os.Symlink("..2026_05_24_09_06_08.123456", filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	// The flat key symlink pointing into ..data.
	if err := os.Symlink("..data/SLACK_WEBHOOK_URL", filepath.Join(dir, "SLACK_WEBHOOK_URL")); err != nil {
		t.Fatal(err)
	}

	m, err := loadSecretDir(dir)
	if err != nil {
		t.Fatalf("loadSecretDir: %v", err)
	}
	if got := m["SLACK_WEBHOOK_URL"]; got != "https://hooks.slack.com/x" {
		t.Errorf("got %q", got)
	}
	if _, ok := m["..data"]; ok {
		t.Error("..data should be ignored")
	}
}

func TestLoadSecretDirMissing(t *testing.T) {
	m, err := loadSecretDir("/nonexistent/path/for/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty, got %v", m)
	}
}
