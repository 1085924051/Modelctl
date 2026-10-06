package ui

import (
	"encoding/json"
	"errors"
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

func TestFormatConnectionErrorExplainsWindowsDaemonStartup(t *testing.T) {
	err := errors.New("Get \\\"http://127.0.0.1:11435/health\\\": dial tcp 127.0.0.1:11435: connectex: No connection could be made because the target machine actively refused it.")
	got := formatConnectionError("http://127.0.0.1:11435", err)
	for _, expected := range []string{"Offline", "daemon", "modelctl daemon", "127.0.0.1:11435"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("connection message missing %q: %s", expected, got)
		}
	}
}

func TestFormatConnectionErrorPreservesUnexpectedErrors(t *testing.T) {
	err := errors.New("unexpected proxy failure")
	if got := formatConnectionError("http://127.0.0.1:11435", err); got != "Offline: unexpected proxy failure" {
		t.Fatalf("unexpected error message = %q", got)
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
	if got := modelStateText(model, "multilingual"); !strings.HasPrefix(got, "Not downloaded") {
		t.Fatalf("modelStateText = %q", got)
	}
}

func TestBuildQuestionsSupportsMultipleTypes(t *testing.T) {
	definitions := []questionDraft{
		{ID: "refund", Type: "noul", Instructions: "Does the customer request a refund?"},
		{ID: "team", Type: "choice", Instructions: "Which team?", Criteria: "billing: Payments\nsupport: Technical help"},
		{ID: "priority", Type: "score", Instructions: "How urgent?", Criteria: "Low\nMedium\nHigh"},
	}
	questions, err := buildQuestions(definitions)
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 3 {
		t.Fatalf("questions = %#v", questions)
	}
	if _, ok := questions["team"].Criteria.(map[string]string); !ok {
		t.Fatalf("choice criteria = %#v", questions["team"].Criteria)
	}
	if levels, ok := questions["priority"].Criteria.([]string); !ok || len(levels) != 3 {
		t.Fatalf("score criteria = %#v", questions["priority"].Criteria)
	}
	if questions["refund"].Criteria != nil {
		t.Fatalf("noul criteria = %#v", questions["refund"].Criteria)
	}
}

func TestBuildQuestionsRejectsDuplicateIDsAndMalformedOptions(t *testing.T) {
	cases := [][]questionDraft{
		{{ID: "same", Type: "noul", Instructions: "One"}, {ID: "same", Type: "noul", Instructions: "Two"}},
		{{ID: "choice", Type: "choice", Instructions: "Which?", Criteria: "missing separator"}},
		{{ID: "score", Type: "score", Instructions: "How much?", Criteria: ""}},
	}
	for _, definitions := range cases {
		if _, err := buildQuestions(definitions); err == nil {
			t.Fatalf("expected error for %#v", definitions)
		}
	}
}

func TestMergeModelStatesPreservesSupportedSelection(t *testing.T) {
	previous := []modelState{{summary: api.ModelSummary{ID: "laya"}, variant: "multilingual", profile: "mps"}}
	next := []modelState{{summary: api.ModelSummary{ID: "laya"}, detail: api.ModelDetail{
		Variants:  []api.Variant{{ID: "english", Default: true}, {ID: "multilingual"}},
		Preflight: api.Preflight{Profiles: []api.Profile{{ID: "auto", Supported: true}, {ID: "mps", Supported: true}}},
	}, variant: "english", profile: "auto"}}
	merged := mergeModelStates(previous, next)
	if merged[0].variant != "multilingual" || merged[0].profile != "mps" {
		t.Fatalf("selection was reset: %#v", merged[0])
	}
}

func TestActiveInstancesExcludesStoppedAndFailedHistory(t *testing.T) {
	instances := []api.Instance{{Status: "ready"}, {Status: "starting"}, {Status: "stopped"}, {Status: "failed"}}
	active := activeInstances(instances)
	if len(active) != 2 || active[0].Status != "ready" || active[1].Status != "starting" {
		t.Fatalf("activeInstances = %#v", active)
	}
}

func TestFormatResultIncludesProbabilitiesAndRawJSON(t *testing.T) {
	result := api.SystemOneResponse{Answers: map[string]api.Answer{
		"team": {Type: "choice", Choice: "billing", Confidence: 0.87, Probabilities: map[string]float64{"billing": 0.9, "support": 0.1}},
	}}
	formatted := formatResult(result)
	for _, expected := range []string{"team", "billing", "87%", "90.0%"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("result missing %q: %s", expected, formatted)
		}
	}
	if strings.Contains(formatted, `"answers"`) {
		t.Fatalf("raw JSON should be separate from summary: %s", formatted)
	}
	if raw := formatRawResult(result); !strings.Contains(raw, `"answers"`) {
		t.Fatalf("raw JSON missing answers: %s", raw)
	}
}

func TestAnswerValuePrefersNormalizedValue(t *testing.T) {
	if got := answerValue(api.Answer{Type: "noul", Value: 0.92, Noul: 0.41}); got != "0.92" {
		t.Fatalf("normalized noul value = %q", got)
	}
	if got := answerValue(api.Answer{Type: "choice", Value: "billing", Choice: "support"}); got != "billing" {
		t.Fatalf("normalized choice value = %q", got)
	}
}

func TestCapabilitySnippetPinsVersionAndMetadata(t *testing.T) {
	snippet := capabilitySnippet("http://127.0.0.1:11435", "refund-check", "1.2.0", map[string]any{"type": "text", "field": "text"}, true)
	for _, expected := range []string{"refund-check/invoke?version=1.2.0", "MODELCTL_API_TOKEN", "ticket_id", "1.2.0"} {
		if !strings.Contains(snippet, expected) {
			t.Fatalf("integration snippet missing %q: %s", expected, snippet)
		}
	}
}

func TestBuildCapabilityVersionSelectionsIncludesPublishedHistory(t *testing.T) {
	selections := buildCapabilityVersionSelections([]api.Capability{
		{ID: "refund-check", Version: "1.1.0", AvailableVersions: []string{"1.0.0", "1.1.0"}},
		{ID: "ticket-routing", Version: "1.0.0"},
	})
	if len(selections) != 3 {
		t.Fatalf("selections = %#v", selections)
	}
	if selections[0].ID != "refund-check" || selections[0].Version != "1.0.0" || selections[1].Version != "1.1.0" {
		t.Fatalf("versions were not sorted or preserved: %#v", selections)
	}
	if selections[2].ID != "ticket-routing" {
		t.Fatalf("active-only capability missing: %#v", selections)
	}
}

func TestCapabilityTemplatesCoverCommonBusinessWorkflows(t *testing.T) {
	templates := capabilityTemplates()
	if len(templates) < 3 {
		t.Fatalf("expected starter templates, got %d", len(templates))
	}
	seen := map[string]bool{}
	for _, template := range templates {
		if template.ID == "" || template.Name == "" || template.Fields == "" || template.Input == "" || len(template.Questions) == 0 {
			t.Fatalf("incomplete template: %#v", template)
		}
		if seen[template.ID] {
			t.Fatalf("duplicate template ID: %s", template.ID)
		}
		seen[template.ID] = true
	}
	for _, id := range []string{"refund-check", "ticket-routing", "document-risk"} {
		if !seen[id] {
			t.Fatalf("missing template %q", id)
		}
	}
}

func TestCapabilityTestInputsFollowStructuredContracts(t *testing.T) {
	input := map[string]any{
		"type": "object",
		"fields": map[string]any{
			"text":     map[string]any{"type": "string", "required": true},
			"order_id": map[string]any{"type": "string", "required": false},
			"urgent":   map[string]any{"type": "boolean", "required": true},
		},
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(sampleCapabilityInputJSON(input)), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["text"] != "The customer was charged twice and wants a refund." || decoded["urgent"] != false {
		t.Fatalf("sample input = %#v", decoded)
	}
	if lines := strings.Split(sampleBatchCapabilityInput(input), "\n"); len(lines) != 2 {
		t.Fatalf("batch sample lines = %d", len(lines))
	}
}

func TestCapabilityDefinitionJSONIsPortableForPublish(t *testing.T) {
	definition := capabilityDefinitionJSON(api.Capability{
		ID: "refund-check", Version: "1.2.0", Name: "Refund check", Description: "Check refund intent",
		Model:     api.CapabilityModel{ModelID: "convaiinnovations/laya", Variant: "english"},
		Input:     map[string]any{"type": "text", "field": "text"},
		Questions: map[string]api.Question{"refund": {Type: "noul", Instructions: "Refund?"}},
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(definition), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["id"] != "refund-check" || payload["version"] != "1.2.0" || payload["activate"] != false {
		t.Fatalf("portable definition = %#v", payload)
	}
	if _, ok := payload["active"]; ok {
		t.Fatal("runtime state leaked into portable definition")
	}
	var imported api.Capability
	if err := json.Unmarshal([]byte(definition), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.ID != "refund-check" || imported.Version != "1.2.0" || imported.Model.ModelID == "" || imported.Model.Variant == "" {
		t.Fatalf("portable definition cannot be imported: %#v", imported)
	}
}

func TestBuildCapabilityInputParsesHumanFriendlyFields(t *testing.T) {
	input, err := buildCapabilityInput("text:string:Customer message\norder_id?:string:Order number", "Message: {{text}}\\nOrder: {{order_id}}")
	if err != nil {
		t.Fatal(err)
	}
	fields := input["fields"].(map[string]any)
	if len(fields) != 2 || fields["order_id"].(map[string]any)["type"] != "string" || fields["order_id"].(map[string]any)["required"] != false {
		t.Fatalf("parsed input fields = %#v", fields)
	}
}

func TestParseRawRequestRejectsMissingQuestions(t *testing.T) {
	if _, err := parseRawRequest(`{"state":{"body":"hello"}}`); err == nil {
		t.Fatal("expected missing questions error")
	}
	valid, err := parseRawRequest(`{"state":[{"role":"user","content":"hello"}],"questions":{"topic":{"type":"noul","instructions":"Relevant?"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(valid, &decoded); err != nil || len(decoded["state"]) == 0 || decoded["state"][0] != '[' {
		t.Fatalf("parsed raw request = %s, err=%v", valid, err)
	}
}

func TestParseRawRequestRejectsInvalidCriteriaShape(t *testing.T) {
	cases := []string{
		`{"state":"hello","questions":{"priority":{"type":"score","instructions":"How urgent?","criteria":{"low":"Low"}}}}`,
		`{"state":"hello","questions":{"team":{"type":"choice","instructions":"Which team?","criteria":[]}}}`,
	}
	for _, input := range cases {
		if _, err := parseRawRequest(input); err == nil {
			t.Fatalf("expected invalid criteria error for %s", input)
		}
	}
}

func TestParseRawRequestAcceptsChoiceLabelArray(t *testing.T) {
	input := `{"state":"hello","questions":{"team":{"type":"choice","instructions":"Which team?","criteria":["billing","support"]}}}`
	if _, err := parseRawRequest(input); err != nil {
		t.Fatalf("valid choice labels rejected: %v", err)
	}
}

func TestExampleRequestContainsTwoQuestions(t *testing.T) {
	request, err := parseRawRequest(exampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Questions map[string]json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(request, &payload); err != nil || len(payload.Questions) != 2 {
		t.Fatalf("example questions = %#v, err = %v", payload.Questions, err)
	}
}

func TestFormatScoreShowsLegendAndPreciseProbabilities(t *testing.T) {
	result := api.SystemOneResponse{Answers: map[string]api.Answer{
		"priority": {Type: "score", Score: 1.7889, Legend: map[string]string{"0": "Low", "1": "Medium", "2": "High"}, Probabilities: map[string]float64{"0": 0.0567, "1": 0.0976, "2": 0.8456}},
	}}
	formatted := formatResult(result)
	for _, expected := range []string{"1.7889", "High", "84.6%"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("score result missing %q: %s", expected, formatted)
		}
	}
}

func TestNextQuestionIDSkipsExistingIds(t *testing.T) {
	drafts := []questionDraft{{ID: "question_1"}, {ID: "question_3"}}
	if got := nextQuestionID(drafts); got != "question_2" {
		t.Fatalf("nextQuestionID = %q", got)
	}
}
