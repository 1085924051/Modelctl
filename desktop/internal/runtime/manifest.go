package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const manifestFileName = "runtime-manifest.json"

type Manifest struct {
	SchemaVersion   int               `json:"schema_version"`
	SoftwareVersion string            `json:"software_version"`
	BuildCommit     string            `json:"build_commit"`
	Target          string            `json:"target"`
	NodeVersion     string            `json:"node_version,omitempty"`
	PythonVersion   string            `json:"python_version,omitempty"`
	LayaVersion     string            `json:"laya_version,omitempty"`
	TorchVersion    string            `json:"torch_version,omitempty"`
	Files           map[string]string `json:"files"`
	Licenses        []string          `json:"licenses"`
}

type Paths struct {
	Root             string
	ManifestPath     string
	NodeBinary       string
	ControlPlaneRoot string
	PythonBinary     string
	LogDir           string
}

func targetTriple() string { return runtime.GOOS + "-" + runtime.GOARCH }

func requiredNodeRelativePath() string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(filepath.Join("node", "node.exe"))
	}
	return filepath.ToSlash(filepath.Join("node", "bin", "node"))
}

func requiredControlPlaneRelativePath() string {
	return filepath.ToSlash(filepath.Join("control-plane", "bin", "modelctl.js"))
}

func requiredPythonRelativePath() string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(filepath.Join("python", "python.exe"))
	}
	return filepath.ToSlash(filepath.Join("python", "bin", "python"))
}

func Resolve(appPath string) (Paths, error) {
	if appPath == "" {
		var err error
		appPath, err = os.Executable()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve executable: %w", err)
		}
	}
	appPath, err := filepath.Abs(appPath)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve application path: %w", err)
	}
	dir := filepath.Dir(appPath)
	candidates := []string{filepath.Join(dir, "runtime"), filepath.Join(dir, "..", "runtime"), filepath.Join(dir, "..", "Resources", "runtime")}
	if filepath.Base(dir) == "MacOS" {
		candidates = append([]string{filepath.Join(dir, "..", "Resources", "runtime")}, candidates...)
	}
	for _, candidate := range candidates {
		root, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if fileExists(filepath.Join(root, manifestFileName)) {
			return Paths{
				Root:             root,
				ManifestPath:     filepath.Join(root, manifestFileName),
				NodeBinary:       filepath.Join(root, requiredNodeRelativePath()),
				ControlPlaneRoot: filepath.Join(root, "control-plane"),
				PythonBinary:     filepath.Join(root, requiredPythonRelativePath()),
				LogDir:           filepath.Join(root, "logs"),
			}, nil
		}
	}
	return Paths{}, fmt.Errorf("packaged runtime manifest not found beside %s", appPath)
}

func Verify(paths Paths) error {
	if paths.Root == "" {
		return errors.New("runtime root is empty")
	}
	manifestPath := paths.ManifestPath
	if manifestPath == "" {
		manifestPath = filepath.Join(paths.Root, manifestFileName)
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read runtime manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return fmt.Errorf("parse runtime manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported runtime manifest schema %d", manifest.SchemaVersion)
	}
	if manifest.Target != targetTriple() {
		return fmt.Errorf("runtime target %q does not match host %q", manifest.Target, targetTriple())
	}
	if len(manifest.Files) == 0 {
		return errors.New("runtime manifest contains no files")
	}
	for relative, expected := range manifest.Files {
		normalized := filepath.ToSlash(relative)
		clean := path.Clean(normalized)
		if filepath.IsAbs(relative) || normalized != relative || clean != normalized || clean == "." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("runtime manifest contains unsafe path %q", relative)
		}
		filePath := filepath.Join(paths.Root, filepath.FromSlash(normalized))
		if !fileExists(filePath) {
			return fmt.Errorf("runtime file is missing: %s", relative)
		}
		actual, err := fileSHA256(filePath)
		if err != nil {
			return fmt.Errorf("hash runtime file %s: %w", relative, err)
		}
		if !strings.EqualFold(actual, expected) {
			return fmt.Errorf("runtime file hash mismatch for %s", relative)
		}
	}
	for _, required := range []string{requiredNodeRelativePath(), requiredControlPlaneRelativePath(), requiredPythonRelativePath()} {
		if _, ok := manifest.Files[required]; !ok {
			return fmt.Errorf("runtime manifest does not list required file %s", required)
		}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileSHA256(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}
