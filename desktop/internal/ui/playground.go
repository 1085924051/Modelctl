package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

type questionEditor struct {
	id           *widget.Entry
	kind         *widget.Select
	instructions *widget.Entry
	criteria     *widget.Entry
	criteriaView fyne.CanvasObject
	content      fyne.CanvasObject
}

func (q *questionEditor) draft() questionDraft {
	return questionDraft{ID: q.id.Text, Type: questionType(q.kind.Selected), Instructions: q.instructions.Text, Criteria: q.criteria.Text}
}

type playgroundState struct {
	body      string
	raw       string
	jsonMode  bool
	questions []questionDraft
}

func (d *Desktop) questionLabel(kind string) string {
	switch kind {
	case "choice":
		return d.t("Choose an option", "从选项中选择")
	case "score":
		return d.t("Rate a level", "评定等级")
	default:
		return d.t("Yes / no", "判断是否成立")
	}
}

func questionType(label string) string {
	switch label {
	case "判断是否成立", "Yes / no":
		return "noul"
	case "从选项中选择", "Choose an option":
		return "choice"
	case "评定等级", "Rate a level":
		return "score"
	default:
		return label
	}
}

func (d *Desktop) addQuestion() {
	row := &questionEditor{
		id:           widget.NewEntry(),
		kind:         widget.NewSelect([]string{d.t("Yes / no", "判断是否成立"), d.t("Choose an option", "从选项中选择"), d.t("Rate a level", "评定等级")}, nil),
		instructions: widget.NewEntry(),
		criteria:     widget.NewMultiLineEntry(),
	}
	row.criteriaView = d.pageWheelEntry(row.criteria)
	row.id.SetPlaceHolder(d.t("Unique API field name", "API 字段名"))
	drafts := make([]questionDraft, 0, len(d.questionRows))
	for _, existing := range d.questionRows {
		drafts = append(drafts, existing.draft())
	}
	row.id.SetText(nextQuestionID(drafts))
	row.instructions.SetPlaceHolder(d.t("Example: Is the customer asking for a refund?", "例如：客户是否要求退款？"))
	row.criteria.SetPlaceHolder(d.t("One option per line, for example: Billing", "每行一个选项，例如：账单问题"))
	row.kind.OnChanged = func(selected string) {
		kind := questionType(selected)
		if kind == "noul" {
			row.criteriaView.Hide()
		} else {
			row.criteriaView.Show()
			if kind == "score" {
				row.criteria.SetPlaceHolder(d.t("One level per line, low to high: Low, Medium, High", "每行一个等级，从低到高：低、中、高"))
			} else {
				row.criteria.SetPlaceHolder(d.t("One option per line, for example: Billing", "每行一个选项，例如：账单问题"))
			}
		}
	}
	row.kind.SetSelected(d.t("Yes / no", "判断是否成立"))
	remove := widget.NewButton(d.t("Remove", "删除"), func() {
		if len(d.questionRows) <= 1 {
			d.result.ParseMarkdown(d.t("_Keep at least one question._", "_至少保留一个问题。_"))
			return
		}
		for index, current := range d.questionRows {
			if current == row {
				d.questionRows = append(d.questionRows[:index], d.questionRows[index+1:]...)
				break
			}
		}
		d.questionList.Remove(row.content)
	})
	advanced := widget.NewAccordion(widget.NewAccordionItem(d.t("Advanced: API field name", "高级：API 字段名"), row.id))
	advanced.CloseAll()
	row.content = widget.NewCard(d.t("Business question", "业务问题"), "", container.NewVBox(
		widget.NewLabel(d.t("What should the model decide from the material?", "希望模型根据资料判断什么？请用一句话提问。")),
		row.instructions,
		widget.NewLabel(d.t("Answer format", "答案形式")), row.kind,
		row.criteriaView,
		advanced,
		remove,
	))
	d.questionRows = append(d.questionRows, row)
	d.questionList.Add(row.content)
}

func (d *Desktop) updateInstanceSelect() {
	if d.instanceSelect == nil {
		return
	}
	selected := d.instanceSelect.Selected
	options := make([]string, 0)
	defaultID := ""
	for _, instance := range readyInstances(d.instances) {
		if !contains(instance.Capabilities, "system_one") {
			continue
		}
		options = append(options, instance.ID)
		if instance.Default {
			defaultID = instance.ID
		}
	}
	d.instanceSelect.Options = options
	d.instanceSelect.Refresh()
	if contains(options, selected) {
		return
	}
	if defaultID != "" {
		d.instanceSelect.SetSelected(defaultID)
	} else if len(options) > 0 {
		d.instanceSelect.SetSelected(options[0])
	} else {
		d.instanceSelect.ClearSelected()
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (d *Desktop) buildPlayground() {
	d.instanceSelect = widget.NewSelect(nil, nil)
	d.result = widget.NewRichTextFromMarkdown(d.t("_Run a decision to see results._", "_运行后将在这里看到答案。_"))
	rawResult := widget.NewMultiLineEntry()
	rawResult.Disable()
	rawResult.SetMinRowsVisible(10)
	rawDetails := widget.NewAccordion(widget.NewAccordionItem(d.t("Raw JSON for developers", "开发者：原始 JSON"), d.pageWheelEntry(rawResult)))
	rawDetails.CloseAll()
	d.questionList = container.NewVBox()
	d.questionRows = nil
	d.addQuestion()
	mode := widget.NewRadioGroup([]string{d.t("Guided form", "引导表单"), "JSON"}, nil)
	mode.Horizontal = true
	mode.SetSelected(d.t("Guided form", "引导表单"))
	body := widget.NewMultiLineEntry()
	d.playgroundBody = body
	body.SetPlaceHolder(d.t("Paste a customer message, document, or situation to analyze", "粘贴客户消息、文档片段，或描述需要判断的情况"))
	body.SetMinRowsVisible(5)
	rawInput := widget.NewMultiLineEntry()
	d.playgroundRaw = rawInput
	d.playgroundMode = mode
	rawInput.SetPlaceHolder(`{"state":{"body":"..."},"questions":{"topic":{"type":"noul","instructions":"..."}}}`)
	rawInput.SetMinRowsVisible(12)
	rawInputView := d.pageWheelEntry(rawInput)
	rawInputView.Hide()
	example := widget.NewButton(d.t("Fill example", "填入示例"), func() {
		if mode.Selected == "JSON" {
			rawInput.SetText(exampleRequest())
			return
		}
		body.SetText(d.t("We were charged twice. Please refund us.", "我被重复扣款了，请帮我退款。"))
		for len(d.questionRows) > 1 {
			d.questionList.Remove(d.questionRows[len(d.questionRows)-1].content)
			d.questionRows = d.questionRows[:len(d.questionRows)-1]
		}
		d.questionRows[0].id.SetText("refund")
		d.questionRows[0].instructions.SetText(d.t("Is the customer asking for a refund?", "客户是否要求退款？"))
		d.questionRows[0].kind.SetSelected(d.t("Yes / no", "判断是否成立"))
	})
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	run := widget.NewButton(d.t("Run decision", "开始判断"), nil)
	run.Importance = widget.HighImportance
	runTop := widget.NewButton(d.t("Run decision", "开始判断"), func() { run.OnTapped() })
	runTop.Importance = widget.HighImportance
	busy := false
	updateRun := func() {
		if busy {
			run.Disable()
			runTop.Disable()
			return
		}
		if d.instanceSelect.Selected == "" {
			run.Disable()
			runTop.Disable()
			status.SetText(d.t("Load a model on the Models page first.", "请先到“模型下载”页加载模型。"))
			return
		}
		if mode.Selected != "JSON" && strings.TrimSpace(body.Text) == "" {
			run.Disable()
			runTop.Disable()
			status.SetText(d.t("Add material to analyze, or fill the example.", "请填写待分析资料，或点击“填入示例”。"))
			return
		}
		run.Enable()
		runTop.Enable()
		status.SetText("")
	}
	d.instanceSelect.OnChanged = func(string) { updateRun() }
	body.OnChanged = func(string) { updateRun() }
	run.OnTapped = func() {
		if d.instanceSelect.Selected == "" {
			status.SetText(d.t("Load a model first.", "请先加载模型。"))
			return
		}
		instanceID := d.instanceSelect.Selected
		names := make(map[string]string, len(d.questionRows))
		for _, row := range d.questionRows {
			names[row.id.Text] = row.instructions.Text
		}
		var invoke func() (api.SystemOneResponse, error)
		if mode.Selected == "JSON" {
			request, err := parseRawRequest(rawInput.Text)
			if err != nil {
				status.SetText(err.Error())
				return
			}
			invoke = func() (api.SystemOneResponse, error) { return d.client.InvokeRawSystemOne(instanceID, request) }
		} else {
			if strings.TrimSpace(body.Text) == "" {
				status.SetText(d.t("Add material to analyze.", "请填写待分析资料。"))
				return
			}
			drafts := make([]questionDraft, 0, len(d.questionRows))
			for _, editor := range d.questionRows {
				drafts = append(drafts, editor.draft())
			}
			questions, err := buildQuestions(drafts)
			if err != nil {
				status.SetText(err.Error())
				return
			}
			request := api.SystemOneRequest{State: api.State{Body: strings.TrimSpace(body.Text)}, Questions: questions}
			invoke = func() (api.SystemOneResponse, error) { return d.client.InvokeSystemOne(instanceID, request) }
		}
		busy = true
		run.Disable()
		runTop.Disable()
		status.SetText(d.t("Running…", "正在判断…"))
		go func() {
			response, invokeErr := invoke()
			fyne.Do(func() {
				busy = false
				updateRun()
				if invokeErr != nil {
					status.SetText(invokeErr.Error())
					return
				}
				status.SetText(fmt.Sprintf(d.t("%d answer(s)", "已得到 %d 个答案"), len(response.Answers)))
				d.result.ParseMarkdown(formatPlaygroundResult(response, d.language, names))
				rawResult.SetText(formatRawResult(response))
			})
		}()
	}
	formFields := container.NewVBox(
		widget.NewCard(d.t("1 · Material", "1 · 待分析资料"), d.t("The facts the model should read", "模型将从这段内容里寻找依据"), d.pageWheelEntry(body)),
		container.NewBorder(nil, nil, widget.NewLabelWithStyle(d.t("2 · Business questions", "2 · 业务问题"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewButton(d.t("Add question", "添加问题"), d.addQuestion)),
		d.questionList,
	)
	mode.OnChanged = func(selected string) {
		if selected == "JSON" {
			formFields.Hide()
			rawInputView.Show()
		} else {
			rawInputView.Hide()
			formFields.Show()
		}
		updateRun()
	}
	form := container.NewVBox(
		widget.NewLabelWithStyle(d.t("Try a decision", "试用模型判断"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(d.t("Choose a running model, add material and a business question, then run.", "选择已加载的模型，填入资料和要判断的问题，即可运行。")),
		widget.NewLabel(d.t("Running model", "已加载的模型")), d.instanceSelect,
		container.NewBorder(nil, nil, mode, example),
		container.NewVBox(runTop, status),
		formFields,
		rawInputView,
		run,
	)
	output := widget.NewCard(d.t("Answers", "判断结果"), d.t("Readable answers appear here", "结果将在这里显示"), container.NewVBox(d.result, rawDetails))
	split := container.NewHSplit(form, output)
	split.Offset = 0.55
	d.playground = split
	if saved := d.pendingPlayground; saved != nil {
		d.pendingPlayground = nil
		body.SetText(saved.body)
		rawInput.SetText(saved.raw)
		for index, draft := range saved.questions {
			if index > 0 {
				d.addQuestion()
			}
			row := d.questionRows[index]
			row.id.SetText(draft.ID)
			row.instructions.SetText(draft.Instructions)
			row.criteria.SetText(draft.Criteria)
			row.kind.SetSelected(d.questionLabel(draft.Type))
		}
		if saved.jsonMode {
			mode.SetSelected("JSON")
		}
	}
	updateRun()
}
