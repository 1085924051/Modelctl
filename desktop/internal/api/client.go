package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultURL = "http://127.0.0.1:11435"

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("MODELCTL_URL")
	}
	if baseURL == "" {
		baseURL = defaultURL
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: os.Getenv("MODELCTL_API_TOKEN"), http: &http.Client{Timeout: 120 * time.Second}}
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) SetBaseURL(baseURL string) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// SetToken updates the bearer token used by subsequent requests. The desktop
// settings view keeps it in memory for the current session.
func (c *Client) SetToken(token string) { c.token = strings.TrimSpace(token) }

func (c *Client) HasToken() bool { return c.token != "" }

type ModelSummary struct {
	ID                string   `json:"id"`
	Version           string   `json:"version"`
	Revision          string   `json:"revision"`
	Variants          []string `json:"variants"`
	InstalledVariants []string `json:"installed_variants"`
	Capabilities      []string `json:"capabilities"`
	License           string   `json:"license"`
}

type ModelsResponse struct {
	Items []ModelSummary `json:"items"`
}

type Artifact struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
}

type Variant struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Default     bool       `json:"default"`
	Installed   bool       `json:"installed"`
	Artifacts   []Artifact `json:"artifacts"`
}

type Profile struct {
	ID        string `json:"id"`
	Device    string `json:"device"`
	Supported bool   `json:"supported"`
}

type Preflight struct {
	Platform      string    `json:"platform"`
	FreeDiskBytes int64     `json:"free_disk_bytes"`
	Profiles      []Profile `json:"profiles"`
}

type ModelDetail struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Version     string    `json:"version"`
	Variants    []Variant `json:"variants"`
	Profiles    []Profile `json:"profiles"`
	Preflight   Preflight `json:"preflight"`
}

type ModelRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Variant string `json:"variant"`
}

type InstanceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Instance struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	Model          ModelRef       `json:"model"`
	Runtime        string         `json:"runtime"`
	Profile        string         `json:"profile"`
	Device         string         `json:"device"`
	Host           string         `json:"host"`
	Port           int            `json:"port"`
	Capabilities   []string       `json:"capabilities"`
	Default        bool           `json:"default"`
	LoadedVariants []string       `json:"loaded_variants"`
	Error          *InstanceError `json:"error"`
}

type InstancesResponse struct {
	Items []Instance `json:"items"`
}

type PullRequest struct {
	ModelID string `json:"model_id"`
	Version string `json:"version,omitempty"`
	Variant string `json:"variant,omitempty"`
}

type PullResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Poll   string `json:"poll"`
}

type Task struct {
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Variant  string       `json:"variant"`
	Progress TaskProgress `json:"progress"`
	Error    *APIError    `json:"error"`
}

type TaskProgress struct {
	BytesDone  int64 `json:"bytes_done"`
	BytesTotal int64 `json:"bytes_total"`
	ItemsDone  int   `json:"items_done"`
	ItemsTotal int   `json:"items_total"`
}

type StartInstanceRequest struct {
	ModelID string `json:"model_id"`
	Version string `json:"version,omitempty"`
	Variant string `json:"variant,omitempty"`
	Profile string `json:"profile,omitempty"`
	Default bool   `json:"default"`
}

type State struct {
	Body string `json:"body"`
}

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type SystemOneRequest struct {
	State     State               `json:"state"`
	Questions map[string]Question `json:"questions"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Score         any                `json:"score,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type SystemOneResponse struct {
	Answers map[string]Answer `json:"answers"`
	Usage   map[string]any    `json:"usage,omitempty"`
	Routing map[string]any    `json:"routing,omitempty"`
}

type CapabilityModel struct {
	ModelID string `json:"model_id"`
	Version string `json:"version,omitempty"`
	Variant string `json:"variant"`
	Profile string `json:"profile,omitempty"`
}

type Capability struct {
	SchemaVersion int                    `json:"schema_version"`
	ID            string                 `json:"id"`
	Version       string                 `json:"version"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Model         CapabilityModel        `json:"model"`
	Input         map[string]any         `json:"input"`
	Questions     map[string]Question    `json:"questions"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	Active        bool                   `json:"active"`
	AvailableVersions []string            `json:"available_versions"`
	Activate      *bool                  `json:"activate,omitempty"`
}

type CapabilitiesResponse struct { Items []Capability `json:"items"` }

type CapabilityInvokeRequest struct {
	Input    map[string]any `json:"input"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CapabilityBatchResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Poll   string `json:"poll"`
}

type CapabilitySchema struct {
	Capability    map[string]any `json:"capability"`
	RequestSchema map[string]any `json:"request_schema"`
	ResponseSchema map[string]any `json:"response_schema"`
}

type CapabilityInvokeResponse struct {
	RequestID string         `json:"request_id"`
	Capability map[string]any `json:"capability"`
	Output    map[string]Answer `json:"output"`
	Raw       SystemOneResponse `json:"raw"`
	Model     ModelRef          `json:"model"`
	RunID     string            `json:"run_id"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *Client) Health() error {
	var result struct {
		Status string `json:"status"`
	}
	if err := c.do(http.MethodGet, "/health", nil, &result); err != nil {
		return err
	}
	if result.Status != "ok" {
		return fmt.Errorf("service is not healthy")
	}
	return nil
}

func (c *Client) Models() (ModelsResponse, error) {
	var result ModelsResponse
	err := c.do(http.MethodGet, "/v1/models", nil, &result)
	return result, err
}

func (c *Client) ModelDetail(modelID string) (ModelDetail, error) {
	var result ModelDetail
	err := c.do(http.MethodGet, "/v1/models/"+url.PathEscape(modelID), nil, &result)
	return result, err
}

func (c *Client) Instances() (InstancesResponse, error) {
	var result InstancesResponse
	err := c.do(http.MethodGet, "/v1/instances", nil, &result)
	return result, err
}

func (c *Client) Pull(request PullRequest) (PullResponse, error) {
	var result PullResponse
	err := c.do(http.MethodPost, "/v1/pulls", request, &result)
	return result, err
}

func (c *Client) Task(taskID string) (Task, error) {
	var result Task
	err := c.do(http.MethodGet, "/v1/tasks/"+url.PathEscape(taskID), nil, &result)
	return result, err
}

func (c *Client) StartInstance(request StartInstanceRequest) (Instance, error) {
	var result Instance
	err := c.do(http.MethodPost, "/v1/instances", request, &result)
	return result, err
}

func (c *Client) StopInstance(instanceID string) (Instance, error) {
	var result Instance
	err := c.do(http.MethodDelete, "/v1/instances/"+url.PathEscape(instanceID), nil, &result)
	return result, err
}

func (c *Client) InvokeSystemOne(instanceID string, request SystemOneRequest) (SystemOneResponse, error) {
	var result SystemOneResponse
	path := "/v1/instances/" + url.PathEscape(instanceID) + "/operations/system_one"
	err := c.do(http.MethodPost, path, request, &result)
	return result, err
}

func (c *Client) InvokeRawSystemOne(instanceID string, request json.RawMessage) (SystemOneResponse, error) {
	var result SystemOneResponse
	path := "/v1/instances/" + url.PathEscape(instanceID) + "/operations/system_one"
	err := c.do(http.MethodPost, path, request, &result)
	return result, err
}

type Run struct {
	ID                string            `json:"id"`
	CapabilityID      string            `json:"capability_id"`
	CapabilityVersion string            `json:"capability_version"`
	InstanceID        string            `json:"instance_id"`
	Input             map[string]any    `json:"input"`
	Response          SystemOneResponse `json:"response"`
	CreatedAt         string            `json:"created_at"`
}

type RunsResponse struct { Items []Run `json:"items"` }

func (c *Client) Capabilities() (CapabilitiesResponse, error) {
	var result CapabilitiesResponse
	err := c.do(http.MethodGet, "/v1/capabilities", nil, &result)
	return result, err
}

func (c *Client) CreateCapability(capability Capability) (Capability, error) {
	var result Capability
	err := c.do(http.MethodPost, "/v1/capabilities", capability, &result)
	return result, err
}

func (c *Client) ActivateCapability(id, version string) (Capability, error) {
	var result Capability
	err := c.do(http.MethodPost, "/v1/capabilities/"+url.PathEscape(id)+"/activate", map[string]any{"version": version}, &result)
	return result, err
}

func (c *Client) InvokeCapability(id string, request CapabilityInvokeRequest, version ...string) (CapabilityInvokeResponse, error) {
	var result CapabilityInvokeResponse
	path := "/v1/capabilities/"+url.PathEscape(id)+"/invoke"
	if len(version) > 0 && version[0] != "" { path += "?version=" + url.QueryEscape(version[0]) }
	err := c.do(http.MethodPost, path, request, &result)
	return result, err
}

func (c *Client) CreateCapabilityBatch(id string, items []CapabilityInvokeRequest, version ...string) (CapabilityBatchResponse, error) {
	var result CapabilityBatchResponse
	path := "/v1/capabilities/"+url.PathEscape(id)+"/batch"
	if len(version) > 0 && version[0] != "" { path += "?version=" + url.QueryEscape(version[0]) }
	err := c.do(http.MethodPost, path, map[string]any{"items": items}, &result)
	return result, err
}

func (c *Client) CapabilitySchema(id string, version ...string) (CapabilitySchema, error) {
	var result CapabilitySchema
	path := "/v1/capabilities/"+url.PathEscape(id)+"/schema"
	if len(version) > 0 && version[0] != "" { path += "?version=" + url.QueryEscape(version[0]) }
	err := c.do(http.MethodGet, path, nil, &result)
	return result, err
}

func (c *Client) Runs() (RunsResponse, error) {
	var result RunsResponse
	err := c.do(http.MethodGet, "/v1/runs", nil, &result)
	return result, err
}

func (c *Client) do(method, path string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var envelope struct {
			Error APIError `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&envelope)
		if envelope.Error.Code != "" {
			return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return fmt.Errorf("daemon returned HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(target)
}
