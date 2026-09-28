package main

import (
	"os"

	"github.com/1085924051/modelctl/desktop/internal/api"
	"github.com/1085924051/modelctl/desktop/internal/ui"
)

func main() { ui.New(api.NewClient(os.Getenv("MODELCTL_URL"))).Run() }
