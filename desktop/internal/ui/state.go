package ui

import "github.com/1085924051/modelctl/desktop/internal/api"

func progressPercent(done, total int64) int {
	if total <= 0 {
		return 0
	}
	value := int(done * 100 / total)
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func chooseVariant(variants []api.Variant, selected string) string {
	for _, variant := range variants {
		if variant.ID == selected {
			return selected
		}
	}
	for _, variant := range variants {
		if variant.Default {
			return variant.ID
		}
	}
	if len(variants) > 0 {
		return variants[0].ID
	}
	return ""
}

func supportedProfileIDs(profiles []api.Profile) []string {
	result := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Supported {
			result = append(result, profile.ID)
		}
	}
	return result
}

func installedVariant(model api.ModelSummary, variant string) bool {
	for _, installed := range model.InstalledVariants {
		if installed == variant {
			return true
		}
	}
	return false
}
