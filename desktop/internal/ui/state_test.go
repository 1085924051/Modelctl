package ui

import (
	"strings"
	"testing"

	"github.com/1085924051/modelctl/desktop/internal/api"
)

func TestProgressPercent(t *testing.T) {
	for _, test := range []struct {
		name  string
		done  int64
		total int64
		want  int
	}{
		{"empty", 0, 0, 0},
		{"half", 50, 100, 50},
		{"clamped", 150, 100, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := progressPercent(test.done, test.total); got != test.want {
				t.Fatalf("progressPercent(%d, %d) = %d, want %d", test.done, test.total, got, test.want)
			}
		})
	}
}

func TestChooseVariantAndProfile(t *testing.T) {
	variants := []api.Variant{{ID: "english", Default: true}, {ID: "multilingual", Installed: true}}
	if got := chooseVariant(variants, ""); got != "english" {
		t.Fatalf("default variant = %q", got)
	}
	if got := chooseVariant(variants, "multilingual"); got != "multilingual" {
		t.Fatalf("selected variant = %q", got)
	}
	profiles := []api.Profile{{ID: "auto", Supported: true}, {ID: "cpu", Supported: true}, {ID: "mps", Supported: false}}
	if got := supportedProfileIDs(profiles); len(got) != 2 || got[0] != "auto" || got[1] != "cpu" {
		t.Fatalf("supported profiles = %#v", got)
	}
}

func TestInstalledVariant(t *testing.T) {
	model := api.ModelSummary{InstalledVariants: []string{"english", "multilingual"}}
	if !installedVariant(model, "multilingual") || installedVariant(model, "missing") {
		t.Fatal("installedVariant returned the wrong result")
	}
}

func TestReadyInstancesIgnoresHistory(t *testing.T) {
	instances := []api.Instance{{Status: "stopped"}, {Status: "ready", Model: api.ModelRef{Variant: "multilingual"}}, {Status: "failed"}}
	if got := readyInstances(instances); len(got) != 1 || got[0].Model.Variant != "multilingual" {
		t.Fatalf("readyInstances = %#v", got)
	}
}

func TestModelStateTextTracksSelectedVariant(t *testing.T) {
	model := api.ModelSummary{InstalledVariants: []string{"english"}}
	if got := modelStateText(model, "multilingual"); !strings.HasPrefix(got, "Not installed") {
		t.Fatalf("modelStateText = %q", got)
	}
}

func TestParseCriteria(t *testing.T) {
	got := parseCriteria("billing: refunds\nsupport: technical help")
	if got["billing"] != "refunds" || got["support"] != "technical help" {
		t.Fatalf("parseCriteria = %#v", got)
	}
}
