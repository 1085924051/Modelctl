package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1085924051/modelctl/desktop/internal/api"
)

func parseRawRequest(text string) (json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(body["state"]) == 0 || string(body["state"]) == "null" {
		return nil, fmt.Errorf("JSON needs a state value")
	}
	var questions map[string]json.RawMessage
	if err := json.Unmarshal(body["questions"], &questions); err != nil || len(questions) == 0 {
		return nil, fmt.Errorf("JSON needs a non-empty questions object")
	}
	for id, raw := range questions {
		var question api.Question
		if err := json.Unmarshal(raw, &question); err != nil || strings.TrimSpace(question.Instructions) == "" || !contains([]string{"noul", "choice", "score"}, question.Type) {
			return nil, fmt.Errorf("question %s needs a valid type and instructions", id)
		}
		var definition map[string]json.RawMessage
		if err := json.Unmarshal(raw, &definition); err != nil {
			return nil, fmt.Errorf("question %s must be an object", id)
		}
		criteria := definition["criteria"]
		switch question.Type {
		case "choice":
			if len(criteria) == 0 {
				return nil, fmt.Errorf("question %s needs choice criteria", id)
			}
			if criteria[0] == '{' {
				var options map[string]json.RawMessage
				if json.Unmarshal(criteria, &options) != nil || len(options) == 0 {
					return nil, fmt.Errorf("question %s needs non-empty choice criteria", id)
				}
			} else if criteria[0] == '[' {
				var labels []string
				if json.Unmarshal(criteria, &labels) != nil || len(labels) == 0 {
					return nil, fmt.Errorf("question %s needs non-empty choice labels", id)
				}
			} else {
				return nil, fmt.Errorf("question %s needs choice criteria object or labels array", id)
			}
		case "score":
			var levels []string
			if len(criteria) == 0 || criteria[0] != '[' || json.Unmarshal(criteria, &levels) != nil || len(levels) == 0 {
				return nil, fmt.Errorf("question %s needs a score levels array", id)
			}
		case "noul":
			if len(criteria) > 0 && string(criteria) != "null" && criteria[0] != '{' {
				return nil, fmt.Errorf("question %s needs a true/false criteria object", id)
			}
		}
	}
	return json.RawMessage(text), nil
}

type questionDraft struct {
	ID           string
	Type         string
	Instructions string
	Criteria     string
}

func nextQuestionID(drafts []questionDraft) string {
	used := make(map[string]bool, len(drafts))
	for _, draft := range drafts {
		used[draft.ID] = true
	}
	for index := 1; ; index++ {
		id := fmt.Sprintf("question_%d", index)
		if !used[id] {
			return id
		}
	}
}

func exampleRequest() string {
	request := api.SystemOneRequest{
		State: api.State{Body: "We were charged twice. Please refund us."},
		Questions: map[string]api.Question{
			"refund": {Type: "noul", Instructions: "Does the customer ask for a refund?"},
			"team": {Type: "choice", Instructions: "Which team should handle this?", Criteria: map[string]string{
				"billing": "payments and refunds",
				"support": "technical help",
			}},
		},
	}
	payload, _ := json.MarshalIndent(request, "", "  ")
	return string(payload)
}

func buildQuestions(drafts []questionDraft) (map[string]api.Question, error) {
	if len(drafts) == 0 {
		return nil, fmt.Errorf("add at least one question")
	}
	questions := make(map[string]api.Question, len(drafts))
	for _, draft := range drafts {
		id := strings.TrimSpace(draft.ID)
		instructions := strings.TrimSpace(draft.Instructions)
		if id == "" || instructions == "" {
			return nil, fmt.Errorf("each question needs an ID and instructions")
		}
		if _, exists := questions[id]; exists {
			return nil, fmt.Errorf("duplicate question ID: %s", id)
		}
		question := api.Question{Type: draft.Type, Instructions: instructions}
		switch draft.Type {
		case "noul":
		case "choice":
			choices := make(map[string]string)
			for _, line := range strings.Split(draft.Criteria, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				key, value, ok := strings.Cut(line, ":")
				key, value = strings.TrimSpace(key), strings.TrimSpace(value)
				if !ok || key == "" || value == "" {
					return nil, fmt.Errorf("%s: use key: description for each choice", id)
				}
				if _, exists := choices[key]; exists {
					return nil, fmt.Errorf("%s: duplicate choice %s", id, key)
				}
				choices[key] = value
			}
			if len(choices) == 0 {
				return nil, fmt.Errorf("%s: add at least one choice", id)
			}
			question.Criteria = choices
		case "score":
			levels := make([]string, 0)
			for _, line := range strings.Split(draft.Criteria, "\n") {
				if level := strings.TrimSpace(line); level != "" {
					levels = append(levels, level)
				}
			}
			if len(levels) == 0 {
				return nil, fmt.Errorf("%s: add at least one score level", id)
			}
			question.Criteria = levels
		default:
			return nil, fmt.Errorf("%s: unsupported question type %q", id, draft.Type)
		}
		questions[id] = question
	}
	return questions, nil
}
