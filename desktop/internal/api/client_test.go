package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReadsModelsAndInstances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(ModelsResponse{Items: []ModelSummary{{ID: "convaiinnovations/laya", Variants: []string{"english", "multilingual"}, InstalledVariants: []string{"multilingual"}}}})
		case "/v1/instances":
			_ = json.NewEncoder(w).Encode(InstancesResponse{Items: []Instance{{ID: "inst_1", Status: "ready", Model: ModelRef{Variant: "multilingual"}, Device: "mps", Default: true}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	models, err := client.Models()
	if err != nil || len(models.Items) != 1 || models.Items[0].InstalledVariants[0] != "multilingual" {
		t.Fatalf("Models() = %#v, %v", models, err)
	}
	instances, err := client.Instances()
	if err != nil || len(instances.Items) != 1 || instances.Items[0].Device != "mps" {
		t.Fatalf("Instances() = %#v, %v", instances, err)
	}
}

func TestClientPullAndInvokePayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/pulls" && r.Method == http.MethodPost {
			var body PullRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ModelID != "convaiinnovations/laya" || body.Variant != "english" {
				t.Fatalf("pull body = %#v, err=%v", body, err)
			}
			_ = json.NewEncoder(w).Encode(PullResponse{TaskID: "task_1", Status: "queued"})
			return
		}
		if r.URL.Path == "/v1/instances/inst_1/operations/system_one" && r.Method == http.MethodPost {
			var body SystemOneRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.State.Body != "refund" {
				t.Fatalf("invoke body = %#v, err=%v", body, err)
			}
			_ = json.NewEncoder(w).Encode(SystemOneResponse{Answers: map[string]Answer{"refund": {Type: "noul", Noul: 0.9}}})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	if result, err := client.Pull(PullRequest{ModelID: "convaiinnovations/laya", Variant: "english"}); err != nil || result.TaskID != "task_1" {
		t.Fatalf("Pull() = %#v, %v", result, err)
	}
	result, err := client.InvokeSystemOne("inst_1", SystemOneRequest{State: State{Body: "refund"}, Questions: map[string]Question{"refund": {Type: "noul", Instructions: "refund?"}}})
	if err != nil || result.Answers["refund"].Noul != 0.9 {
		t.Fatalf("InvokeSystemOne() = %#v, %v", result, err)
	}
}

func TestClientReturnsStructuredHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "MODEL_NOT_INSTALLED", "message": "pull first"}})
	}))
	defer server.Close()
	_, err := NewClient(server.URL).Models()
	if err == nil || err.Error() != "MODEL_NOT_INSTALLED: pull first" {
		t.Fatalf("error = %v", err)
	}
}

func TestSystemOneRequestEncodesScoreLevelsAsArray(t *testing.T) {
	request := SystemOneRequest{
		State: State{Body: "The customer needs a refund."},
		Questions: map[string]Question{
			"department": {Type: "choice", Instructions: "Which team?", Criteria: map[string]string{"billing": "Payments", "support": "Technical help"}},
			"priority":   {Type: "score", Instructions: "How urgent?", Criteria: []string{"Low", "Medium", "High"}},
		},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Questions map[string]struct {
			Criteria json.RawMessage `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Questions["priority"].Criteria) == 0 || decoded.Questions["priority"].Criteria[0] != '[' {
		t.Fatalf("score criteria must be an array: %s", decoded.Questions["priority"].Criteria)
	}
	if len(decoded.Questions["department"].Criteria) == 0 || decoded.Questions["department"].Criteria[0] != '{' {
		t.Fatalf("choice criteria must be an object: %s", decoded.Questions["department"].Criteria)
	}
}

func TestInvokeRawSystemOnePreservesConversationState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/inst_1/operations/system_one" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			State     []map[string]string        `json:"state"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.State) != 2 || len(body.Questions) != 1 {
			t.Fatalf("raw request changed: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(SystemOneResponse{Answers: map[string]Answer{"urgent": {Type: "noul", Noul: 0.8}}})
	}))
	defer server.Close()
	raw := json.RawMessage(`{"state":[{"role":"user","content":"refund"},{"role":"assistant","content":"checking"}],"questions":{"urgent":{"type":"noul","instructions":"Urgent?"}}}`)
	result, err := NewClient(server.URL).InvokeRawSystemOne("inst_1", raw)
	if err != nil || result.Answers["urgent"].Noul != 0.8 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
