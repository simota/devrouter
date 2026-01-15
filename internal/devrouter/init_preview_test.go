package devrouter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewInitConfig_MissingCompose_ExistingOutput(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"name":"app","scripts":{"dev":"pnpm dev"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "devrouter.yaml"), []byte("stack: app\n"), 0o644); err != nil {
		t.Fatalf("write devrouter.yaml: %v", err)
	}

	result, err := PreviewInitConfig(InitPreviewOptions{RepoPath: dir})
	if err != nil {
		t.Fatalf("preview init config: %v", err)
	}
	if result.ApplyAllowed {
		t.Fatalf("ApplyAllowed = true, want false")
	}
	if !result.RequiresForce {
		t.Fatalf("RequiresForce = false, want true")
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), warningComposeMissing) {
		t.Fatalf("warning missing: %v", result.Warnings)
	}
}
