package ui

import (
	"encoding/json"
	"fmt"
	"strings"

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
	name := widget.NewEntry()
	name.SetPlaceHolder("Refund request check")
	description := widget.NewMultiLineEntry()
	description.SetPlaceHolder("Decide whether a customer is explicitly asking for a refund.")
	modelOptions := make([]string, 0, len(d.models))
	modelByLabel := make(map[string]modelState)
	for _, model := range d.models {
		label := model.summary.ID + " · " + model.variant
		modelOptions = append(modelOptions, label)
		modelByLabel[label] = model
	}
	modelSelect := widget.NewSelect(modelOptions, nil)
	if len(modelOptions) > 0 { modelSelect.SetSelected(modelOptions[0]) }
	questionID := widget.NewEntry()
	questionID.SetText("decision")
	questionType := widget.NewSelect([]string{"noul", "choice", "score"}, nil)
	questionType.SetSelected("noul")
	instructions := widget.NewEntry()
	instructions.SetPlaceHolder("What should the model decide?")
	criteria := widget.NewMultiLineEntry()
	criteria.SetPlaceHolder("choice: one key: description per line; score: one level per line")
	status := widget.NewLabel("")

	create := widget.NewButton("Publish capability", func() {
		selected, ok := modelByLabel[modelSelect.Selected]
		if !ok { status.SetText("Load a model before publishing a capability."); return }
		questions, err := buildCapabilityQuestion(questionID.Text, questionType.Selected, instructions.Text, criteria.Text)
		if err != nil { status.SetText(err.Error()); return }
		capability := api.Capability{
			ID: strings.TrimSpace(id.Text), Name: strings.TrimSpace(name.Text), Description: strings.TrimSpace(description.Text),
			Version: "1.0.0", SchemaVersion: 1,
			Model: api.CapabilityModel{ModelID: selected.summary.ID, Version: selected.summary.Version, Variant: selected.variant, Profile: selected.profile},
			Input: map[string]any{"type": "text", "field": "text"}, Questions: questions,
		}
		if capability.ID == "" || capability.Name == "" { status.SetText("ID and name are required."); return }
		created, err := d.client.CreateCapability(capability)
		if err != nil { status.SetText(err.Error()); return }
		d.capabilities = append(d.capabilities, created)
		status.SetText("Published " + created.ID + " v" + created.Version)
		d.renderCapabilities()
	})

	form := widget.NewCard("Publish a capability", "Create a business-facing contract", container.NewVBox(
		widget.NewLabel("ID"), id,
		widget.NewLabel("Display name"), name,
		widget.NewLabel("Description"), description,
		widget.NewLabel("Model"), modelSelect,
		widget.NewLabel("Question ID"), questionID,
		widget.NewLabel("Question type"), questionType,
		widget.NewLabel("Instructions"), instructions,
		widget.NewLabel("Criteria"), criteria,
		container.NewHBox(create, status),
	))
	box.Add(form)

	if len(d.capabilities) == 0 {
		box.Add(widget.NewLabel("No capabilities published yet. Create one above, then copy the generated API example."))
	} else {
		box.Add(widget.NewLabelWithStyle("Published capabilities", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, capability := range d.capabilities {
			capability := capability
			testInput := widget.NewMultiLineEntry()
			testInput.SetText("The customer was charged twice and wants a refund.")
			testInput.SetMinRowsVisible(3)
			testStatus := widget.NewLabel("")
			var testButton *widget.Button
			testButton = widget.NewButton("Test capability", func() {
				testButton.Disable()
				testStatus.SetText("Running…")
				go func() {
					result, err := d.client.InvokeCapability(capability.ID, api.CapabilityInvokeRequest{Input: map[string]any{"text": strings.TrimSpace(testInput.Text)}})
					fyne.Do(func() {
						testButton.Enable()
						if err != nil { testStatus.SetText(err.Error()); return }
						payload, _ := json.MarshalIndent(result.Output, "", "  ")
						testStatus.SetText(string(payload))
					})
				}()
			})
			box.Add(widget.NewCard(capability.Name, capability.ID+" v"+capability.Version, container.NewVBox(
				widget.NewLabel(capability.Description),
				widget.NewLabel(fmt.Sprintf("Model: %s · %s · %s", capability.Model.ModelID, capability.Model.Variant, capability.Model.Profile)),
				widget.NewLabel("Test input"),
				testInput,
				container.NewHBox(testButton, testStatus),
				widget.NewLabel("Integration example"),
				widget.NewLabel(capabilitySnippet(d.client.BaseURL(), capability.ID)),
			)))
		}
	}
	d.page.Content = container.NewVScroll(box)
	d.page.Refresh()
}

func buildCapabilityQuestion(id, kind, instructions, criteria string) (map[string]api.Question, error) {
	id = strings.TrimSpace(id); instructions = strings.TrimSpace(instructions)
	if id == "" || instructions == "" { return nil, fmt.Errorf("question ID and instructions are required") }
	question := api.Question{Type: kind, Instructions: instructions}
	if kind == "choice" {
		items := map[string]string{}
		for _, line := range strings.Split(criteria, "\n") { key, value, ok := strings.Cut(line, ":"); if strings.TrimSpace(line) == "" { continue }; if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" { return nil, fmt.Errorf("choice criteria must use key: description") }; items[strings.TrimSpace(key)] = strings.TrimSpace(value) }
		if len(items) == 0 { return nil, fmt.Errorf("add at least one choice") }; question.Criteria = items
	} else if kind == "score" {
		levels := []string{}; for _, line := range strings.Split(criteria, "\n") { if value := strings.TrimSpace(line); value != "" { levels = append(levels, value) } }; if len(levels) == 0 { return nil, fmt.Errorf("add at least one score level") }; question.Criteria = levels
	}
	return map[string]api.Question{id: question}, nil
}

func capabilitySnippet(baseURL, id string) string {
	payload, _ := json.Marshal(map[string]any{"input": map[string]string{"text": "your text here"}})
	endpoint := fmt.Sprintf("%s/v1/capabilities/%s/invoke", baseURL, id)
	return fmt.Sprintf("POST %s\nContent-Type: application/json\n\n%s\n\nPython: requests.post(%q, json=%s)", endpoint, string(payload), endpoint, string(payload))
}
