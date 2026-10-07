package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/1085924051/modelctl/desktop/internal/api"
)

// formatPlaygroundResult uses the user's business questions instead of API IDs.
func formatPlaygroundResult(result api.SystemOneResponse, language string, names map[string]string) string {
	local := func(en, zh string) string {
		if language == "en" {
			return en
		}
		return zh
	}
	keys := make([]string, 0, len(result.Answers))
	for key := range result.Answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		answer := result.Answers[key]
		title := strings.TrimSpace(names[key])
		if title == "" {
			title = key
		}
		kind := answer.Type
		switch kind {
		case "noul":
			kind = local("Yes / no probability", "成立概率")
		case "choice":
			kind = local("Selected option", "选中选项")
		case "score":
			kind = local("Rating", "等级评分")
		}
		value := answerValue(answer)
		if answer.Type == "choice" && answer.Legend[value] != "" {
			value = answer.Legend[value]
		}
		if answer.Type == "noul" {
			if number, ok := answer.Value.(float64); ok {
				value = fmt.Sprintf("%.1f%%", number*100)
			} else if answer.Value == nil {
				value = fmt.Sprintf("%.1f%%", answer.Noul*100)
			}
		}
		lines = append(lines, fmt.Sprintf("### %s\n%s: **%s** · %s", title, local("Answer", "答案"), value, kind))
		if answer.Confidence > 0 {
			lines = append(lines, fmt.Sprintf("%s: %.0f%%", local("Confidence", "置信度"), answer.Confidence*100))
		}
		if len(answer.Probabilities) > 0 {
			options := make([]string, 0, len(answer.Probabilities))
			for label := range answer.Probabilities {
				options = append(options, label)
			}
			sort.Slice(options, func(i, j int) bool { return answer.Probabilities[options[i]] > answer.Probabilities[options[j]] })
			for _, option := range options {
				label := option
				if answer.Legend[option] != "" {
					label = answer.Legend[option]
				}
				lines = append(lines, fmt.Sprintf("- %s: %.1f%%", label, answer.Probabilities[option]*100))
			}
		}
	}
	return strings.Join(lines, "\n\n")
}
