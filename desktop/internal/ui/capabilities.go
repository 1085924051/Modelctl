package ui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

// renderCapabilities is the business-facing layer above the raw Laya protocol.
// It lets a user publish a named capability and gives developers a copyable API contract.
func (d *Desktop) renderCapabilities() {
	box := container.NewVBox(
		widget.NewLabelWithStyle("Capabilities", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Publish a stable business capability. Your application calls this name instead of building Laya JSON."),
	)

	id := widget.NewEntry()
	id.SetPlaceHolder("refund-check")
	version := widget.NewEntry()
	version.SetText("1.0.0")
	name := widget.NewEntry()
	name.SetPlaceHolder("Refund request check")
	description := widget.NewMultiLineEntry()
	description.SetPlaceHolder("Decide whether a customer is explicitly asking for a refund.")
	inputFields := widget.NewMultiLineEntry()
	inputFields.SetText("text:string:Customer message")
	inputFields.SetMinRowsVisible(2)
	inputTemplate := widget.NewMultiLineEntry()
	inputTemplate.SetText("Customer message: {{text}}")
	inputTemplate.SetMinRowsVisible(2)
	modelOptions := make([]string, 0, len(d.models))
	modelByLabel := make(map[string]modelState)
	for _, model := range d.models {
		label := model.summary.ID + " · " + model.variant
		modelOptions = append(modelOptions, label)
		modelByLabel[label] = model
	}
	modelSelect := widget.NewSelect(modelOptions, nil)
	if len(modelOptions) > 0 {
		modelSelect.SetSelected(modelOptions[0])
	}
	questionRows := []*capabilityQuestionEditor{}
	questionList := container.NewVBox()
	addQuestion := func() {
		row := newCapabilityQuestionEditor(len(questionRows) + 1)
		questionRows = append(questionRows, row)
		questionList.Add(row.view)
	}
	addQuestion()
	status := widget.NewLabel("")
	activate := widget.NewCheck("Activate immediately", nil)
	activate.SetChecked(true)

	create := widget.NewButton("Publish capability", func() {
		selected, ok := modelByLabel[modelSelect.Selected]
		if !ok {
			status.SetText("Load a model before publishing a capability.")
			return
		}
		questions, err := buildCapabilityQuestions(questionRows)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		input, err := buildCapabilityInput(inputFields.Text, inputTemplate.Text)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		capability := api.Capability{
			ID: strings.TrimSpace(id.Text), Name: strings.TrimSpace(name.Text), Description: strings.TrimSpace(description.Text),
			Version: strings.TrimSpace(version.Text), SchemaVersion: 1,
			Model: api.CapabilityModel{ModelID: selected.summary.ID, Version: selected.summary.Version, Variant: selected.variant, Profile: selected.profile},
			Input: input, Questions: questions,
		}
		activateValue := activate.Checked
		capability.Activate = &activateValue
		if capability.ID == "" || capability.Name == "" {
			status.SetText("ID and name are required.")
			return
		}
		if capability.Version == "" {
			status.SetText("Version is required.")
			return
		}
		created, err := d.client.CreateCapability(capability)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		d.capabilities = append(d.capabilities, created)
		status.SetText("Published " + created.ID + " v" + created.Version)
		d.renderCapabilities()
	})

	form := widget.NewCard("Publish a capability", "Create a business-facing contract", container.NewVBox(
		widget.NewLabel("ID"), id,
		widget.NewLabel("Version"), version,
		widget.NewLabel("Display name"), name,
		widget.NewLabel("Description"), description,
		widget.NewLabel("Input fields (id:type:description; add ? after id for optional)"), inputFields,
		widget.NewLabel("Input template (use {{field}} placeholders)"), inputTemplate,
		widget.NewLabel("Model"), modelSelect,
		container.NewBorder(nil, nil, widget.NewLabel("Questions"), widget.NewButton("Add question", addQuestion)),
		questionList,
		activate,
		container.NewHBox(create, status),
	))
	box.Add(form)

	if len(d.capabilities) == 0 {
		box.Add(widget.NewLabel("No capabilities published yet. Create one above, then copy the generated API example."))
	} else {
		box.Add(widget.NewLabelWithStyle("Published capabilities", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, capability := range d.capabilities {
			capability := capability
			snippet := capabilitySnippet(d.client.BaseURL(), capability.ID, capability.Version, capability.Input, d.client.HasToken())
			integration := widget.NewMultiLineEntry()
			integration.SetText(snippet)
			integration.SetMinRowsVisible(11)
			integration.Disable()
			copySnippet := widget.NewButton("Copy integration example", func() { d.app.Clipboard().SetContent(snippet) })
			schemaOutput := widget.NewMultiLineEntry()
			schemaOutput.SetText("Click View schema to load the request and response contract.")
			schemaOutput.SetMinRowsVisible(10)
			schemaOutput.Disable()
			schemaStatus := widget.NewLabel("")
			var viewSchema *widget.Button
			viewSchema = widget.NewButton("View schema", func() {
				viewSchema.Disable()
				schemaStatus.SetText("Loading…")
				go func() {
					schema, err := d.client.CapabilitySchema(capability.ID, capability.Version)
					fyne.Do(func() {
						viewSchema.Enable()
						if err != nil {
							schemaStatus.SetText(err.Error())
							return
						}
						payload, _ := json.MarshalIndent(schema, "", "  ")
						schemaOutput.SetText(string(payload))
						schemaStatus.SetText("Schema loaded")
					})
				}()
			})
			versionStatus := widget.NewLabel("Active: " + capability.Version)
			versionButtons := container.NewHBox()
			for _, version := range capability.AvailableVersions {
				version := version
				if version == capability.Version {
					continue
				}
				var button *widget.Button
				button = widget.NewButton("Activate "+version, func() {
					button.Disable()
					go func() {
						_, err := d.client.ActivateCapability(capability.ID, version)
						fyne.Do(func() {
							if err != nil {
								versionStatus.SetText(err.Error())
							} else {
								versionStatus.SetText("Active: " + version)
								d.refresh()
							}
						})
					}()
				})
				versionButtons.Add(button)
			}
			testInput := widget.NewMultiLineEntry()
			testInput.SetText("The customer was charged twice and wants a refund.")
			testInput.SetMinRowsVisible(3)
			testMetadata := widget.NewMultiLineEntry()
			testMetadata.SetText("{}")
			testMetadata.SetMinRowsVisible(2)
			testStatus := widget.NewLabel("")
			var testButton *widget.Button
			testButton = widget.NewButton("Test capability", func() {
				testButton.Disable()
				var metadata map[string]any
				if err := json.Unmarshal([]byte(testMetadata.Text), &metadata); err != nil || metadata == nil {
					testButton.Enable()
					testStatus.SetText("Metadata must be a JSON object.")
					return
				}
				testStatus.SetText("Running…")
				go func() {
					result, err := d.client.InvokeCapability(capability.ID, api.CapabilityInvokeRequest{Input: map[string]any{"text": strings.TrimSpace(testInput.Text)}, Metadata: metadata})
					fyne.Do(func() {
						testButton.Enable()
						if err != nil {
							testStatus.SetText(err.Error())
							return
						}
						payload, _ := json.MarshalIndent(result.Output, "", "  ")
						testStatus.SetText(string(payload))
					})
				}()
			})
			batchInput := widget.NewMultiLineEntry()
			batchInput.SetText("The customer was charged twice and wants a refund.\nThe user cannot log in to the dashboard.")
			batchInput.SetMinRowsVisible(3)
			batchStatus := widget.NewLabel("")
			var batchButton *widget.Button
			batchButton = widget.NewButton("Run batch", func() {
				items := []api.CapabilityInvokeRequest{}
				for _, line := range strings.Split(batchInput.Text, "\n") {
					if text := strings.TrimSpace(line); text != "" {
						items = append(items, api.CapabilityInvokeRequest{Input: map[string]any{"text": text}})
					}
				}
				if len(items) == 0 {
					batchStatus.SetText("Add at least one input line.")
					return
				}
				batchButton.Disable()
				batchStatus.SetText("Queueing…")
				go func() {
					task, err := d.client.CreateCapabilityBatch(capability.ID, items)
					if err != nil {
						fyne.Do(func() { batchButton.Enable(); batchStatus.SetText(err.Error()) })
						return
					}
					for {
						current, taskErr := d.client.Task(task.TaskID)
						if taskErr != nil {
							fyne.Do(func() { batchButton.Enable(); batchStatus.SetText(taskErr.Error()) })
							return
						}
						fyne.Do(func() {
							batchStatus.SetText(fmt.Sprintf("%s · %d/%d", current.Status, current.Progress.ItemsDone, current.Progress.ItemsTotal))
						})
						if current.Status == "succeeded" || current.Status == "failed" || current.Status == "cancelled" {
							fyne.Do(func() { batchButton.Enable() })
							d.refresh()
							return
						}
						time.Sleep(500 * time.Millisecond)
					}
				}()
			})
			box.Add(widget.NewCard(capability.Name, capability.ID+" v"+capability.Version, container.NewVBox(
				widget.NewLabel(capability.Description),
				widget.NewLabel(fmt.Sprintf("Model: %s · %s · %s", capability.Model.ModelID, capability.Model.Variant, capability.Model.Profile)),
				widget.NewButton("Open Integration", func() { d.showPage("integration") }),
				versionStatus,
				versionButtons,
				container.NewHBox(viewSchema, schemaStatus),
				widget.NewAccordion(widget.NewAccordionItem("Schema", schemaOutput)),
				widget.NewLabel("Test input"),
				testInput,
				widget.NewLabel("Test metadata (JSON object)"),
				testMetadata,
				container.NewHBox(testButton, testStatus),
				widget.NewLabel("Batch test · one input per line"),
				batchInput,
				container.NewHBox(batchButton, batchStatus),
				widget.NewLabel("Integration example"),
				copySnippet,
				integration,
			)))
		}
	}
	d.page.Content = box
	d.page.Refresh()
}

type capabilityQuestionEditor struct {
	id           *widget.Entry
	kind         *widget.Select
	instructions *widget.Entry
	criteria     *widget.Entry
	view         fyne.CanvasObject
}

func newCapabilityQuestionEditor(index int) *capabilityQuestionEditor {
	row := &capabilityQuestionEditor{
		id: widget.NewEntry(), kind: widget.NewSelect([]string{"noul", "choice", "score"}, nil),
		instructions: widget.NewEntry(), criteria: widget.NewMultiLineEntry(),
	}
	row.id.SetText(fmt.Sprintf("decision_%d", index))
	row.instructions.SetPlaceHolder("What should the model decide?")
	row.criteria.SetPlaceHolder("choice: one key: description per line; score: one level per line")
	row.kind.SetSelected("noul")
	row.kind.OnChanged = func(kind string) {
		if kind == "noul" {
			row.criteria.Hide()
		} else {
			row.criteria.Show()
		}
	}
	row.view = widget.NewCard("Question "+fmt.Sprint(index), "", container.NewVBox(
		container.NewGridWithColumns(2, widget.NewLabel("ID"), row.id, widget.NewLabel("Type"), row.kind),
		widget.NewLabel("Instructions"), row.instructions,
		widget.NewLabel("Criteria"), row.criteria,
	))
	return row
}

func buildCapabilityQuestions(rows []*capabilityQuestionEditor) (map[string]api.Question, error) {
	questions := make(map[string]api.Question, len(rows))
	for _, row := range rows {
		id, kind, instructions := strings.TrimSpace(row.id.Text), row.kind.Selected, strings.TrimSpace(row.instructions.Text)
		if id == "" || instructions == "" {
			return nil, fmt.Errorf("each question needs an ID and instructions")
		}
		if _, exists := questions[id]; exists {
			return nil, fmt.Errorf("duplicate question ID: %s", id)
		}
		one, err := buildCapabilityQuestion(id, kind, instructions, row.criteria.Text)
		if err != nil {
			return nil, err
		}
		for key, question := range one {
			questions[key] = question
		}
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("add at least one question")
	}
	return questions, nil
}

func buildCapabilityInput(definitionText, template string) (map[string]any, error) {
	fields := map[string]any{}
	for _, line := range strings.Split(definitionText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("input fields must use id:type:description")
		}
		id, kind := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		required := true
		if strings.HasSuffix(id, "?") {
			required = false
			id = strings.TrimSpace(strings.TrimSuffix(id, "?"))
		}
		description := ""
		if len(parts) == 3 {
			description = strings.TrimSpace(parts[2])
		}
		if id == "" || kind == "" || !contains([]string{"string", "number", "boolean"}, kind) {
			return nil, fmt.Errorf("input field %q needs type string, number, or boolean", id)
		}
		if _, exists := fields[id]; exists {
			return nil, fmt.Errorf("duplicate input field %q", id)
		}
		definition := map[string]any{"type": kind, "required": required}
		if description != "" {
			definition["description"] = description
		}
		fields[id] = definition
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("add at least one input field")
	}
	template = strings.TrimSpace(template)
	if template == "" {
		return nil, fmt.Errorf("input template is required")
	}
	return map[string]any{"type": "object", "fields": fields, "template": template}, nil
}

func buildCapabilityQuestion(id, kind, instructions, criteria string) (map[string]api.Question, error) {
	id = strings.TrimSpace(id)
	instructions = strings.TrimSpace(instructions)
	if id == "" || instructions == "" {
		return nil, fmt.Errorf("question ID and instructions are required")
	}
	question := api.Question{Type: kind, Instructions: instructions}
	if kind == "choice" {
		items := map[string]string{}
		for _, line := range strings.Split(criteria, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("choice criteria must use key: description")
			}
			items[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("add at least one choice")
		}
		question.Criteria = items
	} else if kind == "score" {
		levels := []string{}
		for _, line := range strings.Split(criteria, "\n") {
			if value := strings.TrimSpace(line); value != "" {
				levels = append(levels, value)
			}
		}
		if len(levels) == 0 {
			return nil, fmt.Errorf("add at least one score level")
		}
		question.Criteria = levels
	}
	return map[string]api.Question{id: question}, nil
}

func capabilitySnippet(baseURL, id, version string, inputContract map[string]any, authenticated bool) string {
	payload, _ := json.Marshal(map[string]any{
		"input":    sampleCapabilityInput(inputContract),
		"metadata": map[string]string{"ticket_id": "T-100"},
	})
	endpoint := fmt.Sprintf("%s/v1/capabilities/%s/invoke?version=%s", baseURL, id, url.QueryEscape(version))
	requestHeaders := "Content-Type: application/json"
	if authenticated {
		requestHeaders = "Authorization: Bearer $MODELCTL_API_TOKEN\n" + requestHeaders
	}
	inputJSON, _ := json.Marshal(sampleCapabilityInput(inputContract))
	return fmt.Sprintf("REST\nPOST %s\n%s\n\n%s\n\nPython SDK\nclient.invoke(%q, %s, {\"ticket_id\": \"T-100\"}, %q)\n\nJavaScript SDK\nawait client.invoke(%q, %s, { ticket_id: \"T-100\" }, %q)", endpoint, requestHeaders, string(payload), id, string(inputJSON), version, id, string(inputJSON), version)
}

func sampleCapabilityInput(inputContract map[string]any) map[string]any {
	if fields, ok := inputContract["fields"].(map[string]any); ok {
		result := make(map[string]any, len(fields))
		for id, raw := range fields {
			definition, _ := raw.(map[string]any)
			switch definition["type"] {
			case "number":
				result[id] = 0
			case "boolean":
				result[id] = false
			default:
				result[id] = "your text here"
			}
		}
		return result
	}
	field := "text"
	if value, ok := inputContract["field"].(string); ok && value != "" {
		field = value
	}
	return map[string]any{field: "your text here"}
}
