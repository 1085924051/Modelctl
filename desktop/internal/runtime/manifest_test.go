package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsRuntimeBesideExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "Modelctl")
	if err := os.WriteFile(executable, []byte("client"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeRoot := filepath.Join(root, "runtime")
	if err := os.MkdirAll(filepath.Join(runtimeRoot, "node"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "runtime-manifest.json"), []byte(`{"schema_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := Resolve(executable)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Root != runtimeRoot {
		t.Fatalf("runtime root = %q, want %q", paths.Root, runtimeRoot)
	}
}

func TestResolveFindsAppImageRuntime(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "usr", "bin", "modelctl-desktop")
	runtimeRoot := filepath.Join(root, "usr", "share", "modelctl", "runtime")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("client"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "runtime-manifest.json"), []byte(`{"schema_version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := Resolve(executable)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Root != runtimeRoot {
		t.Fatalf("runtime root = %q, want %q", paths.Root, runtimeRoot)
	}
	if paths.LogDir != "" {
		t.Fatalf("resolved runtime should not write logs into package: %q", paths.LogDir)
	}
}

func TestVerifyAcceptsValidRuntime(t *testing.T) {
	root := writeFixtureRuntime(t, targetTriple())
	paths := Paths{Root: root, ManifestPath: filepath.Join(root, "runtime-manifest.json")}
	if err := Verify(paths); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsTargetMismatch(t *testing.T) {
	root := writeFixtureRuntime(t, "not-this-platform")
	paths := Paths{Root: root, ManifestPath: filepath.Join(root, "runtime-manifest.json")}
	if err := Verify(paths); err == nil {
		t.Fatal("Verify() accepted a mismatched target")
	}
}

func TestVerifyRejectsHashMismatch(t *testing.T) {
	root := writeFixtureRuntime(t, targetTriple())
	if err := os.WriteFile(filepath.Join(root, requiredNodeRelativePath()), []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := Paths{Root: root, ManifestPath: filepath.Join(root, "runtime-manifest.json")}
	if err := Verify(paths); err == nil {
		t.Fatal("Verify() accepted a changed runtime file")
	}
}

func writeFixtureRuntime(t *testing.T, target string) string {
	t.Helper()
	root := t.TempDir()
	for _, relative := range []string{requiredNodeRelativePath(), requiredControlPlaneRelativePath(), requiredPythonRelativePath()} {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(relative), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{}
	for _, relative := range []string{requiredNodeRelativePath(), requiredControlPlaneRelativePath(), requiredPythonRelativePath()} {
		body, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(body)
		files[filepath.ToSlash(relative)] = hex.EncodeToString(hash[:])
	}
	writeManifest(t, root, target, files)
	return root
}

func writeManifest(t *testing.T, root, target string, files map[string]string) {
	t.Helper()
	body := []byte(`{"schema_version":1,"software_version":"0.1.0","build_commit":"test","target":"` + target + `","files":{` + manifestFilesJSON(files) + `},"licenses":[]}`)
	if err := os.WriteFile(filepath.Join(root, "runtime-manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func manifestFilesJSON(files map[string]string) string {
	result := ""
	for relative, hash := range files {
		if result != "" {
			result += ","
		}
		result += `"` + filepath.ToSlash(relative) + `":"` + hash + `"`
	}
	return result
}
