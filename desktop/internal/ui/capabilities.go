package ui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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
		widget.NewLabelWithStyle(d.t("Capabilities", "业务能力"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(d.t("Publish a stable business capability. Your application calls this name instead of building Laya JSON.", "把模型判断发布成稳定的业务接口。应用按能力名称调用，无需构造 Laya JSON。")),
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
	templates := capabilityTemplates()
	templateNames := []string{d.t("Start from a template", "从业务模板开始")}
	templateByName := make(map[string]capabilityTemplate, len(templates))
	for _, template := range templates {
		templateNames = append(templateNames, template.Name)
		templateByName[template.Name] = template
	}
	templateSelect := widget.NewSelect(templateNames, nil)
	templateSelect.SetSelected(templateNames[0])
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
	setQuestions := func(drafts []capabilityTemplateQuestion) {
		questionRows = nil
		questionList.RemoveAll()
		for index, draft := range drafts {
			row := newCapabilityQuestionEditor(d, index+1)
			row.id.SetText(draft.ID)
			row.kind.SetSelected(d.questionLabel(draft.Type))
			row.instructions.SetText(draft.Instructions)
			row.criteria.SetText(draft.Criteria)
			questionRows = append(questionRows, row)
			questionList.Add(row.view)
		}
		questionList.Refresh()
	}
	addQuestion := func() {
		row := newCapabilityQuestionEditor(d, len(questionRows)+1)
		questionRows = append(questionRows, row)
		questionList.Add(row.view)
	}
	setQuestions([]capabilityTemplateQuestion{{ID: "refund", Type: "noul", Instructions: "Does the customer explicitly ask for a refund?"}})
	templateSelect.OnChanged = func(templateName string) {
		template, ok := templateByName[templateName]
		if !ok {
			return
		}
		id.SetText(template.ID)
		version.SetText(template.Version)
		name.SetText(template.DisplayName)
		description.SetText(template.Description)
		inputFields.SetText(template.Fields)
		inputTemplate.SetText(template.Input)
		setQuestions(template.Questions)
	}
	status := widget.NewLabel("")
	activate := widget.NewCheck(d.t("Activate immediately", "发布后立即启用"), nil)
	activate.SetChecked(true)

	var create *widget.Button
	create = widget.NewButton(d.t("Publish capability", "发布业务能力"), func() {
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
		create.Disable()
		status.SetText(d.t("Publishing…", "正在发布…"))
		go func() {
			created, createErr := d.client.CreateCapability(capability)
			fyne.Do(func() {
				create.Enable()
				if createErr != nil {
					status.SetText(createErr.Error())
					return
				}
				d.capabilities = append(d.capabilities, created)
				status.SetText(d.t("Published ", "已发布 ") + created.ID + " v" + created.Version)
				if d.selectedPage == "capabilities" {
					d.renderCapabilities()
				}
			})
		}()
	})
	create.Importance = widget.HighImportance

	form := widget.NewCard(d.t("Publish a capability", "发布业务能力"), d.t("Create a business-facing contract", "创建可供应用调用的稳定接口"), container.NewVBox(
		widget.NewLabel(d.t("Business template", "业务模板")),
		templateSelect,
		widget.NewLabel(d.t("Start with a common workflow, then adjust its fields and decisions.", "先选择常见业务场景，再按需调整字段和问题。")),
		widget.NewLabel(d.t("ID", "接口 ID")), id,
		widget.NewLabel(d.t("Version", "版本")), version,
		widget.NewLabel(d.t("Display name", "显示名称")), name,
		widget.NewLabel(d.t("Description", "用途说明")), d.pageWheelEntry(description),
		widget.NewLabel(d.t("Input fields (id:type:description; add ? after id for optional)", "输入字段（每行 id:类型:说明；可选字段在 id 后加 ?）")), d.pageWheelEntry(inputFields),
		widget.NewLabel(d.t("Input template (use {{field}} placeholders)", "输入模板（用 {{字段名}} 插入业务数据）")), d.pageWheelEntry(inputTemplate),
		widget.NewLabel(d.t("Model", "使用模型")), modelSelect,
		container.NewBorder(nil, nil, widget.NewLabel(d.t("Questions", "业务问题")), widget.NewButton(d.t("Add question", "添加问题"), addQuestion)),
		questionList,
		activate,
		container.NewHBox(create, status),
	))
	createPanel := widget.NewAccordion(widget.NewAccordionItem(d.t("Create a capability", "新建业务能力"), form))
	if len(d.capabilities) == 0 {
		createPanel.Open(0)
	}
	box.Add(createPanel)

	importPath := widget.NewEntry()
	importPath.SetPlaceHolder("Path to a .capability.json file")
	importStatus := widget.NewLabel("")
	var importButton *widget.Button
	importButton = widget.NewButton(d.t("Import capability JSON", "导入能力 JSON"), func() {
		path := strings.TrimSpace(importPath.Text)
		if path == "" {
			importStatus.SetText("Choose or enter a capability JSON file path.")
			return
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			importStatus.SetText(err.Error())
			return
		}
		var imported api.Capability
		if err := json.Unmarshal(payload, &imported); err != nil {
			importStatus.SetText("Invalid capability JSON: " + err.Error())
			return
		}
		if strings.TrimSpace(imported.ID) == "" || strings.TrimSpace(imported.Version) == "" || imported.Model.ModelID == "" || imported.Model.Variant == "" {
			importStatus.SetText("Capability JSON needs id, version, model.model_id, and model.variant.")
			return
		}
		activate := false
		imported.Activate = &activate
		importButton.Disable()
		importStatus.SetText("Publishing imported capability…")
		go func() {
			created, createErr := d.client.CreateCapability(imported)
			fyne.Do(func() {
				importButton.Enable()
				if createErr != nil {
					importStatus.SetText(createErr.Error())
					return
				}
				importStatus.SetText("Imported " + created.ID + " v" + created.Version + ". Activate it after testing.")
				d.refresh()
			})
		}()
	})
	box.Add(widget.NewAccordion(widget.NewAccordionItem(d.t("Import a capability", "导入业务能力"), widget.NewCard(d.t("Import a capability", "导入业务能力"), d.t("Move a reviewed capability definition between machines or environments", "在不同电脑或环境之间迁移已审核的能力配置"), container.NewVBox(
		widget.NewLabel(d.t("Exported capability JSON is portable and contains no model files or secrets.", "导出的能力 JSON 可迁移，不包含模型文件或密钥。")),
		importPath,
		container.NewHBox(importButton, importStatus),
	)))))

	if len(d.capabilities) == 0 {
		box.Add(widget.NewLabel(d.t("No capabilities published yet. Create one above, then copy the generated API example.", "尚未发布业务能力。先用上方表单创建，之后即可复制 API 调用示例。")))
	} else {
		box.Add(widget.NewLabelWithStyle(d.t("Published capabilities", "已发布的业务能力"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, capability := range d.capabilities {
			capability := capability
			snippet := capabilitySnippet(d.client.BaseURL(), capability.ID, capability.Version, capability.Input, d.client.HasToken())
			definitionJSON := capabilityDefinitionJSON(capability)
			integration := widget.NewMultiLineEntry()
			integration.SetText(snippet)
			integration.SetMinRowsVisible(11)
			integration.Disable()
			copySnippet := widget.NewButton(d.t("Copy integration example", "复制接入示例"), func() { d.app.Clipboard().SetContent(snippet) })
			copyDefinition := widget.NewButton(d.t("Copy capability JSON", "复制能力 JSON"), func() { d.app.Clipboard().SetContent(definitionJSON) })
			schemaOutput := widget.NewMultiLineEntry()
			schemaOutput.SetText("Click View schema to load the request and response contract.")
			schemaOutput.SetMinRowsVisible(10)
			schemaOutput.Disable()
			schemaStatus := widget.NewLabel("")
			var viewSchema *widget.Button
			viewSchema = widget.NewButton(d.t("View schema", "查看接口格式"), func() {
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
			versionStatus := widget.NewLabel(d.t("Active: ", "当前启用：") + capability.Version)
			versionButtons := container.NewHBox()
			for _, version := range capability.AvailableVersions {
				version := version
				if version == capability.Version {
					continue
				}
				var button *widget.Button
				button = widget.NewButton(d.t("Activate ", "启用版本 ")+version, func() {
					button.Disable()
					go func() {
						_, err := d.client.ActivateCapability(capability.ID, version)
						fyne.Do(func() {
							if err != nil {
								versionStatus.SetText(err.Error())
							} else {
								versionStatus.SetText(d.t("Active: ", "当前启用：") + version)
								d.refresh()
							}
						})
					}()
				})
				versionButtons.Add(button)
			}
			testInput := widget.NewMultiLineEntry()
			testInput.SetText(sampleCapabilityInputJSON(capability.Input))
			testInput.SetMinRowsVisible(3)
			testMetadata := widget.NewMultiLineEntry()
			testMetadata.SetText("{}")
			testMetadata.SetMinRowsVisible(2)
			testStatus := widget.NewLabel("")
			var testButton *widget.Button
			testButton = widget.NewButton(d.t("Test capability", "测试能力"), func() {
				testButton.Disable()
				var input map[string]any
				if err := json.Unmarshal([]byte(testInput.Text), &input); err != nil || input == nil {
					testButton.Enable()
					testStatus.SetText("Test input must be a JSON object matching the capability fields.")
					return
				}
				var metadata map[string]any
				if err := json.Unmarshal([]byte(testMetadata.Text), &metadata); err != nil || metadata == nil {
					testButton.Enable()
					testStatus.SetText("Metadata must be a JSON object.")
					return
				}
				testStatus.SetText("Running…")
				go func() {
					result, err := d.client.InvokeCapability(capability.ID, api.CapabilityInvokeRequest{Input: input, Metadata: metadata})
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
			batchInput.SetText(sampleBatchCapabilityInput(capability.Input))
			batchInput.SetMinRowsVisible(3)
			batchStatus := widget.NewLabel("")
			var batchButton *widget.Button
			batchButton = widget.NewButton(d.t("Run batch", "运行批量测试"), func() {
				items := []api.CapabilityInvokeRequest{}
				for _, line := range strings.Split(batchInput.Text, "\n") {
					if text := strings.TrimSpace(line); text != "" {
						var input map[string]any
						if err := json.Unmarshal([]byte(text), &input); err != nil || input == nil {
							batchStatus.SetText("Each batch line must be a JSON object matching the capability fields.")
							return
						}
						items = append(items, api.CapabilityInvokeRequest{Input: input})
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
			readinessStatus := widget.NewLabel(d.t("Readiness not checked", "尚未检查部署状态"))
			downloadButton := widget.NewButton(d.t("Download model", "下载模型"), nil)
			downloadButton.Hide()
			downloadButton.OnTapped = func() {
				var selected *modelState
				for index := range d.models {
					candidate := &d.models[index]
					if candidate.summary.ID == capability.Model.ModelID {
						selected = candidate
						break
					}
				}
				if selected == nil {
					readinessStatus.SetText("Model is not in the local catalog. Refresh Models first.")
					return
				}
				downloadButton.Disable()
				readinessStatus.SetText("Downloading model…")
				go func(summary api.ModelSummary, variant string) {
					err := d.pullAndWait(summary, variant, func(progress string) {
						fyne.Do(func() { readinessStatus.SetText(progress) })
					})
					fyne.Do(func() {
						downloadButton.Enable()
						if err != nil {
							readinessStatus.SetText(err.Error())
							return
						}
						downloadButton.Hide()
						readinessStatus.SetText("Model downloaded. Check readiness again or run a test.")
						d.refresh()
					})
				}(selected.summary, capability.Model.Variant)
			}
			var readinessButton *widget.Button
			readinessButton = widget.NewButton(d.t("Check deployment readiness", "检查部署状态"), func() {
				readinessButton.Disable()
				readinessStatus.SetText("Checking…")
				go func() {
					result, err := d.client.CapabilityStatus(capability.ID, capability.Version)
					fyne.Do(func() {
						readinessButton.Enable()
						if err != nil {
							readinessStatus.SetText(err.Error())
							return
						}
						state, _ := result["status"].(string)
						next, _ := result["next_action"].(string)
						if state == "model_not_installed" {
							downloadButton.Show()
							readinessStatus.SetText(fmt.Sprintf("Model %s is not installed. Open Models to download %s.", capability.Model.ModelID, capability.Model.Variant))
							return
						}
						downloadButton.Hide()
						readinessStatus.SetText("Status: " + state + " · next: " + next)
					})
				}()
			})
			advanced := widget.NewAccordion(
				widget.NewAccordionItem(d.t("Schema", "请求与响应格式"), container.NewVBox(container.NewHBox(viewSchema, schemaStatus), d.pageWheelEntry(schemaOutput))),
				widget.NewAccordionItem(d.t("Batch test", "批量测试"), container.NewVBox(widget.NewLabel(d.t("One JSON object per line", "每行一个 JSON 对象")), d.pageWheelEntry(batchInput), container.NewHBox(batchButton, batchStatus))),
				widget.NewAccordionItem(d.t("Integration example", "接入示例"), container.NewVBox(copySnippet, d.pageWheelEntry(integration))),
			)
			advanced.CloseAll()
			box.Add(widget.NewCard(capability.Name, capability.ID+" v"+capability.Version, container.NewVBox(
				widget.NewLabel(capability.Description),
				widget.NewLabel(fmt.Sprintf("Model: %s · %s · %s", capability.Model.ModelID, capability.Model.Variant, capability.Model.Profile)),
				container.NewHBox(widget.NewButton(d.t("Open Integration", "查看接入方式"), func() { d.showPage("integration") }), readinessButton, downloadButton, readinessStatus),
				copyDefinition,
				versionStatus,
				versionButtons,
				widget.NewLabel(d.t("Test input (JSON object matching the fields)", "测试输入（与业务字段对应的 JSON 对象）")),
				d.pageWheelEntry(testInput),
				widget.NewAccordion(widget.NewAccordionItem(d.t("Advanced: test metadata", "高级：测试元数据"), d.pageWheelEntry(testMetadata))),
				container.NewHBox(testButton, testStatus),
				advanced,
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
	criteriaView fyne.CanvasObject
	view         fyne.CanvasObject
}

func newCapabilityQuestionEditor(d *Desktop, index int) *capabilityQuestionEditor {
	row := &capabilityQuestionEditor{
		id: widget.NewEntry(), kind: widget.NewSelect([]string{d.questionLabel("noul"), d.questionLabel("choice"), d.questionLabel("score")}, nil),
		instructions: widget.NewEntry(), criteria: widget.NewMultiLineEntry(),
	}
	row.criteriaView = d.pageWheelEntry(row.criteria)
	row.id.SetText(fmt.Sprintf("decision_%d", index))
	row.instructions.SetPlaceHolder(d.t("What should the model decide?", "希望模型判断什么？"))
	row.criteria.SetPlaceHolder(d.t("One option or level per line", "每行一个选项或等级"))
	row.kind.OnChanged = func(selected string) {
		if questionType(selected) == "noul" {
			row.criteriaView.Hide()
		} else {
			row.criteriaView.Show()
		}
	}
	row.kind.SetSelected(d.questionLabel("noul"))
	row.view = widget.NewCard(fmt.Sprintf(d.t("Question %d", "问题 %d"), index), "", container.NewVBox(
		container.NewGridWithColumns(2, widget.NewLabel(d.t("API field name", "API 字段名")), row.id, widget.NewLabel(d.t("Answer format", "答案形式")), row.kind),
		widget.NewLabel(d.t("Business question", "业务问题")), row.instructions,
		widget.NewLabel(d.t("Options / levels", "选项 / 等级")), row.criteriaView,
	))
	return row
}

func buildCapabilityQuestions(rows []*capabilityQuestionEditor) (map[string]api.Question, error) {
	questions := make(map[string]api.Question, len(rows))
	for _, row := range rows {
		id, kind, instructions := strings.TrimSpace(row.id.Text), questionType(row.kind.Selected), strings.TrimSpace(row.instructions.Text)
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

func sampleCapabilityInputJSON(inputContract map[string]any) string {
	payload, _ := json.MarshalIndent(structuredCapabilitySampleInput(inputContract, "The customer was charged twice and wants a refund."), "", "  ")
	return string(payload)
}

func sampleBatchCapabilityInput(inputContract map[string]any) string {
	first := structuredCapabilitySampleInput(inputContract, "The customer was charged twice and wants a refund.")
	second := structuredCapabilitySampleInput(inputContract, "The user cannot log in to the dashboard.")
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	return string(firstJSON) + "\n" + string(secondJSON)
}

func structuredCapabilitySampleInput(inputContract map[string]any, defaultText string) map[string]any {
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
				if id == "text" || id == "document" {
					result[id] = defaultText
				} else {
					result[id] = "your text here"
				}
			}
		}
		return result
	}
	field := "text"
	if value, ok := inputContract["field"].(string); ok && value != "" {
		field = value
	}
	return map[string]any{field: defaultText}
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
			if !ok {
				key, value = fmt.Sprintf("option_%d", len(items)+1), line
			}
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("each choice needs a name")
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

func capabilityDefinitionJSON(capability api.Capability) string {
	definition := map[string]any{
		"schema_version": 1,
		"id":             capability.ID,
		"version":        capability.Version,
		"name":           capability.Name,
		"description":    capability.Description,
		"model":          capability.Model,
		"input":          capability.Input,
		"questions":      capability.Questions,
		"activate":       false,
	}
	payload, _ := json.MarshalIndent(definition, "", "  ")
	return string(payload)
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
