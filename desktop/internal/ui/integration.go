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
		widget.NewLabelWithStyle(d.t("Integration", "接入应用"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(d.t("Connect your order system, helpdesk, ERP, or application to a published capability. The integration contract hides model and Laya/Jev details.", "将订单系统、客服系统、ERP 或其他应用连接到已发布的业务能力。应用无需了解模型或 Laya/Jev 协议。")),
	)
	if len(d.capabilities) == 0 {
		box.Add(widget.NewCard(d.t("No published capabilities", "还没有已发布的业务能力"), d.t("Publish one first", "请先发布能力"), container.NewVBox(
			widget.NewLabel(d.t("Create and test a capability in Capabilities, then return here to copy the production request.", "先在“业务能力”页创建并测试，随后回来复制应用调用示例。")),
			widget.NewButton(d.t("Open Capabilities", "打开业务能力"), func() { d.showPage("capabilities") }),
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
	box.Add(widget.NewCard(d.t("Choose a capability", "选择业务能力"), d.t("Production callers should pin a version", "生产环境建议固定版本"), container.NewVBox(selectCapability, status)))
	box.Add(detail)

	load := func(selection capabilityVersionSelection) {
		status.SetText(d.t("Loading integration contract…", "正在加载接入说明…"))
		detail.RemoveAll()
		go func() {
			integration, err := d.client.CapabilityIntegration(selection.ID, selection.Version)
			fyne.Do(func() {
				if err != nil {
					status.SetText(err.Error())
					return
				}
				status.SetText(d.t("Contract ready · pin ", "接口已就绪 · 生产环境请固定 ") + selection.ID + "@" + selection.Version)
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
	tokenEnvironmentVariable := ""
	if authRequired {
		tokenEnvironmentVariable, _ = integration.Auth["environment_variable"].(string)
		if tokenEnvironmentVariable == "" {
			tokenEnvironmentVariable = "MODELCTL_API_TOKEN"
		}
		authText = "Bearer token required · use " + tokenEnvironmentVariable + " in the business service secret store"
	}
	connectionTemplate := "MODELCTL_URL=" + integration.BaseURL + "\n"
	if authRequired {
		connectionTemplate += tokenEnvironmentVariable + "=replace-with-secret-manager-value\n"
	}
	copyConnection := widget.NewButton("Copy connection config", func() {
		d.app.Clipboard().SetContent(connectionTemplate)
	})
	readinessStatus := widget.NewLabel("Readiness not checked")
	var readinessButton *widget.Button
	readinessButton = widget.NewButton("Check readiness", func() {
		readinessButton.Disable()
		readinessStatus.SetText("Checking...")
		go func() {
			result, err := d.client.CapabilityStatus(capabilityID, capabilityVersion)
			fyne.Do(func() {
				readinessButton.Enable()
				if err != nil {
					readinessStatus.SetText(err.Error())
					return
				}
				state, _ := result["status"].(string)
				next, _ := result["next_action"].(string)
				readinessStatus.SetText("Status: " + state + " · next: " + next)
			})
		}()
	})
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
		widget.NewLabel("The copied template contains no real secret."),
		copyConnection,
		container.NewHBox(readinessButton, readinessStatus),
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
