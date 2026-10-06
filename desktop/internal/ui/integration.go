package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

type capabilityVersionSelection struct {
	ID      string
	Version string
	Name    string
}

// renderIntegration is the customer handoff surface: a published capability
// becomes a copyable, versioned contract for a business service.
func (d *Desktop) renderIntegration() {
	box := container.NewVBox(
		widget.NewLabelWithStyle("Integration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Connect your order system, helpdesk, ERP, or application to a published capability. The integration contract hides model and Laya/Jev details."),
	)
	if len(d.capabilities) == 0 {
		box.Add(widget.NewCard("No published capabilities", "Publish one first", container.NewVBox(
			widget.NewLabel("Create and test a capability in Capabilities, then return here to copy the production request."),
			widget.NewButton("Open Capabilities", func() { d.showPage("capabilities") }),
		)))
		d.page.Content = box
		d.page.Refresh()
		return
	}

	selections := buildCapabilityVersionSelections(d.capabilities)
	labels := make([]string, 0, len(selections))
	byLabel := map[string]capabilityVersionSelection{}
	for _, selection := range selections {
		label := fmt.Sprintf("%s  ·  %s", selection.ID, selection.Version)
		labels = append(labels, label)
		byLabel[label] = selection
	}
	sort.Strings(labels)
	selectCapability := widget.NewSelect(labels, nil)
	selectCapability.SetSelected(labels[0])
	status := widget.NewLabel("")
	detail := container.NewVBox()
	box.Add(widget.NewCard("Choose a capability", "Production callers should pin a version", container.NewVBox(selectCapability, status)))
	box.Add(detail)

	load := func(selection capabilityVersionSelection) {
		status.SetText("Loading integration contract…")
		detail.RemoveAll()
		go func() {
			integration, err := d.client.CapabilityIntegration(selection.ID, selection.Version)
			fyne.Do(func() {
				if err != nil {
					status.SetText(err.Error())
					return
				}
				status.SetText("Contract ready · pin " + selection.ID + "@" + selection.Version + " in production")
				populateIntegrationDetail(d, detail, integration)
				d.page.Refresh()
			})
		}()
	}
	selectCapability.OnChanged = func(label string) {
		if selection, ok := byLabel[label]; ok {
			load(selection)
		}
	}
	load(byLabel[labels[0]])

	d.page.Content = box
	d.page.Refresh()
}

func buildCapabilityVersionSelections(capabilities []api.Capability) []capabilityVersionSelection {
	selections := []capabilityVersionSelection{}
	for _, capability := range capabilities {
		versions := append([]string(nil), capability.AvailableVersions...)
		if len(versions) == 0 {
			versions = []string{capability.Version}
		}
		for _, version := range versions {
			if strings.TrimSpace(version) == "" {
				continue
			}
			selections = append(selections, capabilityVersionSelection{ID: capability.ID, Version: version, Name: capability.Name})
		}
	}
	sort.Slice(selections, func(i, j int) bool {
		if selections[i].ID == selections[j].ID {
			return selections[i].Version < selections[j].Version
		}
		return selections[i].ID < selections[j].ID
	})
	return selections
}

func populateIntegrationDetail(d *Desktop, detail *fyne.Container, integration api.CapabilityIntegration) {
	capabilityName, _ := integration.Capability["name"].(string)
	capabilityID, _ := integration.Capability["id"].(string)
	capabilityVersion, _ := integration.Capability["version"].(string)
	authRequired, _ := integration.Auth["required"].(bool)
	authText := "No token required in local loopback mode"
	if authRequired {
		authText = "Bearer token required · use MODELCTL_API_TOKEN in the business service secret store"
	}
	openAPIStatus := widget.NewLabel("")
	var openAPIButton *widget.Button
	openAPIButton = widget.NewButton("Copy OpenAPI JSON", func() {
		openAPIButton.Disable()
		openAPIStatus.SetText("Loading...")
		go func() {
			document, err := d.client.CapabilityOpenAPI(capabilityID, capabilityVersion)
			fyne.Do(func() {
				openAPIButton.Enable()
				if err != nil {
					openAPIStatus.SetText(err.Error())
					return
				}
				payload, marshalErr := json.MarshalIndent(document, "", "  ")
				if marshalErr != nil {
					openAPIStatus.SetText(marshalErr.Error())
					return
				}
				d.app.Clipboard().SetContent(string(payload))
				openAPIStatus.SetText("OpenAPI JSON copied")
			})
		}()
	})
	detail.Add(widget.NewCard(capabilityName, "Business service connection", container.NewVBox(
		widget.NewLabel("Endpoint"),
		copyableText(d, integration.Endpoint),
		widget.NewLabel(authText),
		widget.NewLabel("The endpoint is versioned. Modelctl starts the matching local runtime automatically after the model is installed."),
		container.NewHBox(openAPIButton, openAPIStatus),
	)))

	requestSchema, _ := json.MarshalIndent(integration.RequestSchema, "", "  ")
	responseSchema, _ := json.MarshalIndent(integration.ResponseSchema, "", "  ")
	detail.Add(widget.NewAccordion(
		widget.NewAccordionItem("Request schema", readOnlyCode(string(requestSchema), 10)),
		widget.NewAccordionItem("Response schema", readOnlyCode(string(responseSchema), 10)),
	))

	keys := make([]string, 0, len(integration.Examples))
	for key := range integration.Examples {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		code := integration.Examples[key]
		codeView := readOnlyCode(code, 10)
		copyButton := widget.NewButton("Copy", func() { d.app.Clipboard().SetContent(code) })
		detail.Add(widget.NewCard(strings.ToUpper(key), "Copy this example into the business service", container.NewVBox(copyButton, codeView)))
	}
	if len(integration.Errors) > 0 {
		detail.Add(widget.NewLabel("Expected errors: " + strings.Join(integration.Errors, " · ")))
	}
}

func copyableText(d *Desktop, value string) fyne.CanvasObject {
	entry := widget.NewEntry()
	entry.SetText(value)
	entry.Disable()
	return container.NewBorder(nil, nil, nil, widget.NewButton("Copy", func() { d.app.Clipboard().SetContent(value) }), entry)
}

func readOnlyCode(value string, rows int) *widget.Entry {
	entry := widget.NewMultiLineEntry()
	entry.SetText(value)
	entry.SetMinRowsVisible(rows)
	entry.Disable()
	return entry
}
