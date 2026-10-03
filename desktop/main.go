package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/1085924051/modelctl/desktop/internal/api"
	runtimepkg "github.com/1085924051/modelctl/desktop/internal/runtime"
	"github.com/1085924051/modelctl/desktop/internal/ui"
)

func main() {
	baseURL := os.Getenv("MODELCTL_URL")
	client := api.NewClient(baseURL)
	var supervisor *runtimepkg.Supervisor
	var startupError error
	if baseURL == "" {
		if paths, err := runtimepkg.Resolve(""); err == nil {
			if err := runtimepkg.Verify(paths); err != nil {
				startupError = err
			} else {
				dataDir := os.Getenv("MODELCTL_DATA_DIR")
				if dataDir == "" {
					home, homeErr := os.UserHomeDir()
					if homeErr != nil {
						startupError = homeErr
					} else {
						dataDir = filepath.Join(home, ".modelctl")
					}
				}
				if startupError == nil {
					supervisor = runtimepkg.NewSupervisor(paths, dataDir)
					baseURL, startupError = supervisor.Start(context.Background())
					if startupError == nil {
						client = api.NewClient(baseURL)
					}
				}
			}
		}
	}
	ui.NewWithSupervisor(client, supervisor, startupError).Run()
}
