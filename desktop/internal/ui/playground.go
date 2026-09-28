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
	content      fyne.CanvasObject
}

func (q *questionEditor) draft() questionDraft {
	return questionDraft{ID: q.id.Text, Type: q.kind.Selected, Instructions: q.instructions.Text, Criteria: q.criteria.Text}
}

func (d *Desktop) addQuestion() {
	row := &questionEditor{
		id:           widget.NewEntry(),
		kind:         widget.NewSelect([]string{"noul", "choice", "score"}, nil),
		instructions: widget.NewEntry(),
		criteria:     widget.NewMultiLineEntry(),
	}
	row.id.SetPlaceHolder("Question ID")
	drafts := make([]questionDraft, 0, len(d.questionRows))
	for _, existing := range d.questionRows {
		drafts = append(drafts, existing.draft())
	}
	row.id.SetText(nextQuestionID(drafts))
	row.instructions.SetPlaceHolder("What should Laya decide?")
	row.criteria.SetPlaceHolder("key: description, one per line")
	row.kind.OnChanged = func(kind string) {
		if kind == "noul" {
			row.criteria.Hide()
		} else {
			row.criteria.Show()
			if kind == "score" {
				row.criteria.SetPlaceHolder("One level per line, lowest first")
			} else {
				row.criteria.SetPlaceHolder("key: description, one per line")
			}
		}
	}
	row.kind.SetSelected("noul")
	remove := widget.NewButton("Remove", func() {
		if len(d.questionRows) <= 1 {
			d.result.ParseMarkdown("_Keep at least one question._")
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
	row.content = container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Question"), remove),
		container.NewGridWithColumns(2, row.id, row.kind),
		row.instructions,
		row.criteria,
		widget.NewSeparator(),
	)
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
	d.result = widget.NewRichTextFromMarkdown("_Run a decision to see results._")
	rawResult := widget.NewMultiLineEntry()
	rawResult.Disable()
	rawResult.SetMinRowsVisible(10)
	rawDetails := widget.NewAccordion(widget.NewAccordionItem("Raw JSON", rawResult))
	rawDetails.CloseAll()
	d.questionList = container.NewVBox()
	d.addQuestion()
	mode := widget.NewRadioGroup([]string{"Form", "JSON"}, nil)
	mode.Horizontal = true
	mode.SetSelected("Form")
	body := widget.NewMultiLineEntry()
	body.SetPlaceHolder("Paste text or describe a situation")
	body.SetMinRowsVisible(5)
	rawInput := widget.NewMultiLineEntry()
	rawInput.SetPlaceHolder(`{"state":{"body":"..."},"questions":{"topic":{"type":"noul","instructions":"..."}}}`)
	rawInput.SetMinRowsVisible(12)
	rawInput.Hide()
	example := widget.NewButton("Example", func() { rawInput.SetText(exampleRequest()) })
	status := widget.NewLabel("")
	run := widget.NewButton("Run decision", nil)
	run.OnTapped = func() {
		if d.instanceSelect.Selected == "" {
			status.SetText("Start a Laya instance first.")
			return
		}
		instanceID := d.instanceSelect.Selected
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
				status.SetText("Add text to analyze.")
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
		run.Disable()
		status.SetText("Running…")
		go func() {
			response, invokeErr := invoke()
			fyne.Do(func() {
				run.Enable()
				if invokeErr != nil {
					status.SetText(invokeErr.Error())
					return
				}
				status.SetText(fmt.Sprintf("%d answer(s)", len(response.Answers)))
				d.result.ParseMarkdown(formatResult(response))
				rawResult.SetText(formatRawResult(response))
			})
		}()
	}
	formFields := container.NewVBox(
		widget.NewLabel("Input"), body,
		container.NewBorder(nil, nil, widget.NewLabel("Questions"), widget.NewButton("Add", d.addQuestion)),
		d.questionList,
	)
	mode.OnChanged = func(selected string) {
		if selected == "JSON" {
			formFields.Hide()
			rawInput.Show()
		} else {
			rawInput.Hide()
			formFields.Show()
		}
	}
	form := container.NewVBox(
		widget.NewLabelWithStyle("Decision", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Instance"), d.instanceSelect,
		container.NewBorder(nil, nil, mode, example),
		formFields,
		rawInput,
		container.NewHBox(run, status),
	)
	output := container.NewBorder(widget.NewLabelWithStyle("Answers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, container.NewVScroll(container.NewVBox(d.result, rawDetails)))
	split := container.NewHSplit(container.NewVScroll(form), output)
	split.Offset = 0.55
	d.playground = split
}
