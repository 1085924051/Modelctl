package ui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

type modelState struct {
	summary api.ModelSummary
	detail  api.ModelDetail
	variant string
	profile string
}

type Desktop struct {
	client       *api.Client
	app          fyne.App
	window       fyne.Window
	page         *container.Scroll
	status       *widget.Label
	models       []modelState
	instances    []api.Instance
	selectedPage string
	refreshMu    sync.Mutex
}

func New(client *api.Client) *Desktop {
	application := app.NewWithID("com.modelctl.desktop")
	application.Settings().SetTheme(newTheme())
	desktop := &Desktop{client: client, app: application, selectedPage: "models"}
	desktop.window = application.NewWindow("Modelctl")
	desktop.window.Resize(fyne.NewSize(1120, 760))
	desktop.page = container.NewVScroll(container.NewVBox())
	desktop.window.SetContent(desktop.layout())
	desktop.renderModels()
	return desktop
}

func (d *Desktop) Run() { d.refresh(); d.window.ShowAndRun() }

func (d *Desktop) layout() fyne.CanvasObject {
	d.status = widget.NewLabel("Connecting…")
	refresh := widget.NewButton("Refresh", d.refresh)
	header := container.NewBorder(nil, nil, widget.NewLabel("MODELCTL"), refresh, d.status)
	nav := container.NewVBox(
		widget.NewLabelWithStyle("LOCAL MODEL STUDIO", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewButton("Models", func() { d.showPage("models") }),
		widget.NewButton("Running", func() { d.showPage("running") }),
		widget.NewButton("Playground", func() { d.showPage("playground") }),
		layout.NewSpacer(),
		widget.NewLabel("127.0.0.1 daemon"),
	)
	return container.NewBorder(header, nil, nav, nil, d.page)
}

func (d *Desktop) showPage(page string) {
	d.selectedPage = page
	switch page {
	case "running":
		d.renderRunning()
	case "playground":
		d.renderPlayground()
	default:
		d.renderModels()
	}
}

func (d *Desktop) refresh() {
	if !d.refreshMu.TryLock() {
		return
	}
	go func() {
		defer d.refreshMu.Unlock()
		if err := d.client.Health(); err != nil {
			fyne.Do(func() { d.status.SetText("Offline: " + err.Error()) })
			return
		}
		models, err := d.client.Models()
		if err != nil {
			fyne.Do(func() { d.status.SetText("Models unavailable: " + err.Error()) })
			return
		}
		instances, err := d.client.Instances()
		if err != nil {
			fyne.Do(func() { d.status.SetText("Instances unavailable: " + err.Error()) })
			return
		}
		states := make([]modelState, 0, len(models.Items))
		for _, summary := range models.Items {
			detail, detailErr := d.client.ModelDetail(summary.ID)
			if detailErr != nil {
				continue
			}
			states = append(states, modelState{summary: summary, detail: detail, variant: chooseVariant(detail.Variants, ""), profile: firstSupported(detail.Preflight.Profiles)})
		}
		fyne.Do(func() {
			d.models = states
			d.instances = instances.Items
			d.status.SetText(fmt.Sprintf("Connected · %d model(s) · %d ready instance(s)", len(states), len(readyInstances(instances.Items))))
			d.showPage(d.selectedPage)
		})
	}()
}

func firstSupported(profiles []api.Profile) string {
	for _, profile := range profiles {
		if profile.Supported && profile.ID == "auto" {
			return profile.ID
		}
	}
	for _, profile := range profiles {
		if profile.Supported {
			return profile.ID
		}
	}
	return ""
}

func (d *Desktop) renderModels() {
	box := container.NewVBox(widget.NewLabelWithStyle("Models", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(d.models) == 0 {
		box.Add(widget.NewLabel("No models available"))
	}
	for index := range d.models {
		box.Add(d.modelCard(index))
	}
	d.page.Content = box
	d.page.Refresh()
}

func (d *Desktop) modelCard(index int) fyne.CanvasObject {
	model := &d.models[index]
	variantIDs := make([]string, 0, len(model.detail.Variants))
	for _, variant := range model.detail.Variants {
		variantIDs = append(variantIDs, variant.ID)
	}
	stateLabel := widget.NewLabel(modelStateText(model.summary, model.variant))
	variantSelect := widget.NewSelect(variantIDs, func(selected string) {
		model.variant = selected
		stateLabel.SetText(modelStateText(model.summary, selected))
	})
	variantSelect.SetSelected(model.variant)
	profileSelect := widget.NewSelect(supportedProfileIDs(model.detail.Preflight.Profiles), func(selected string) { model.profile = selected })
	profileSelect.SetSelected(model.profile)
	progress := widget.NewLabel("")
	pull := widget.NewButton("Pull", func() { d.pull(model, progress, stateLabel) })
	var run *widget.Button
	run = widget.NewButton("Run", func() { d.run(model, progress, stateLabel, pull, run) })
	content := container.NewVBox(
		widget.NewLabelWithStyle(model.detail.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("%s · v%s", model.summary.ID, model.summary.Version)),
		stateLabel,
		container.NewGridWithColumns(2, widget.NewLabel("Variant"), variantSelect, widget.NewLabel("Device"), profileSelect),
		container.NewHBox(pull, run, progress),
	)
	return widget.NewCard(model.detail.DisplayName, "Structured decision model", content)
}

func modelStateText(model api.ModelSummary, variant string) string {
	if installedVariant(model, variant) {
		return "Installed · ready to run"
	}
	return "Not installed · Pull downloads the verified checkpoint"
}

func (d *Desktop) pull(model *modelState, progress, stateLabel *widget.Label) {
	go func() {
		result, err := d.client.Pull(api.PullRequest{ModelID: model.summary.ID, Version: model.summary.Version, Variant: model.variant})
		if err != nil {
			fyne.Do(func() { progress.SetText(err.Error()) })
			return
		}
		for {
			task, taskErr := d.client.Task(result.TaskID)
			if taskErr != nil {
				fyne.Do(func() { progress.SetText(taskErr.Error()) })
				return
			}
			fyne.Do(func() {
				progress.SetText(fmt.Sprintf("Downloading %d%%", progressPercent(task.Progress.BytesDone, task.Progress.BytesTotal)))
			})
			if task.Status == "succeeded" {
				fyne.Do(func() { stateLabel.SetText("Installed · ready to run"); progress.SetText("Downloaded") })
				d.refresh()
				return
			}
			if task.Status == "failed" || task.Status == "cancelled" {
				fyne.Do(func() { progress.SetText("Download " + task.Status) })
				return
			}
			time.Sleep(750 * time.Millisecond)
		}
	}()
}

func (d *Desktop) run(model *modelState, progress, stateLabel *widget.Label, pull, run *widget.Button) {
	pull.Disable()
	run.Disable()
	go func() {
		if !installedVariant(model.summary, model.variant) {
			fyne.Do(func() { progress.SetText("Pulling before run…") })
			if err := d.pullAndWait(model.summary, model.variant, func(text string) { fyne.Do(func() { progress.SetText(text) }) }); err != nil {
				fyne.Do(func() { progress.SetText(err.Error()); pull.Enable(); run.Enable() })
				return
			}
		}
		instance, err := d.client.StartInstance(api.StartInstanceRequest{ModelID: model.summary.ID, Version: model.summary.Version, Variant: model.variant, Profile: model.profile, Default: true})
		fyne.Do(func() {
			pull.Enable()
			run.Enable()
			if err != nil {
				progress.SetText(err.Error())
				return
			}
			stateLabel.SetText(fmt.Sprintf("Running · %s · %s", instance.Device, instance.ID[:minInt(12, len(instance.ID))]))
			progress.SetText("Ready")
		})
		d.refresh()
	}()
}

func (d *Desktop) pullAndWait(summary api.ModelSummary, variant string, report func(string)) error {
	result, err := d.client.Pull(api.PullRequest{ModelID: summary.ID, Version: summary.Version, Variant: variant})
	if err != nil {
		return err
	}
	for {
		task, taskErr := d.client.Task(result.TaskID)
		if taskErr != nil {
			return taskErr
		}
		report(fmt.Sprintf("Downloading %d%%", progressPercent(task.Progress.BytesDone, task.Progress.BytesTotal)))
		if task.Status == "succeeded" {
			return nil
		}
		if task.Status == "failed" || task.Status == "cancelled" {
			return fmt.Errorf("download %s", task.Status)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func (d *Desktop) renderRunning() {
	box := container.NewVBox(widget.NewLabelWithStyle("Running", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(d.instances) == 0 {
		box.Add(widget.NewLabel("No running instances"))
	}
	for _, instance := range d.instances {
		instance := instance
		stop := widget.NewButton("Stop", func() { go func() { _, _ = d.client.StopInstance(instance.ID); d.refresh() }() })
		box.Add(widget.NewCard(instance.Model.Variant, fmt.Sprintf("%s · %s", instance.Status, instance.Device), container.NewBorder(nil, nil, widget.NewLabel(instance.ID), stop)))
	}
	d.page.Content = box
	d.page.Refresh()
}

func (d *Desktop) renderPlayground() {
	ready := readyInstances(d.instances)
	filtered := ready[:0]
	for _, instance := range ready {
		if len(instance.Capabilities) > 0 {
			filtered = append(filtered, instance)
		}
	}
	ready = filtered
	instanceIDs := make([]string, 0, len(ready))
	for _, instance := range ready {
		instanceIDs = append(instanceIDs, instance.ID)
	}
	selectInstance := widget.NewSelect(instanceIDs, nil)
	for _, instance := range ready {
		if instance.Default {
			selectInstance.SetSelected(instance.ID)
			break
		}
	}
	if selectInstance.Selected == "" && len(instanceIDs) > 0 {
		selectInstance.SetSelected(instanceIDs[0])
	}
	body := widget.NewMultiLineEntry()
	body.SetPlaceHolder("What should Laya decide?")
	question := widget.NewEntry()
	question.SetPlaceHolder("Question instructions")
	typeSelect := widget.NewSelect([]string{"noul", "choice", "score"}, nil)
	typeSelect.SetSelected("noul")
	criteria := widget.NewMultiLineEntry()
	criteria.SetPlaceHolder("choice: key: description, one per line")
	result := widget.NewRichTextFromMarkdown("_No result yet._")
	run := widget.NewButton("Run decision", func() {
		if selectInstance.Selected == "" {
			result.ParseMarkdown("_Start an instance first._")
			return
		}
		request := api.SystemOneRequest{State: api.State{Body: body.Text}, Questions: map[string]api.Question{"decision": {Type: typeSelect.Selected, Instructions: question.Text}}}
		if typeSelect.Selected == "choice" {
			request.Questions["decision"] = api.Question{Type: "choice", Instructions: question.Text, Criteria: parseCriteria(criteria.Text)}
		}
		go func() {
			response, err := d.client.InvokeSystemOne(selectInstance.Selected, request)
			fyne.Do(func() {
				if err != nil {
					result.ParseMarkdown("**Error:** " + err.Error())
					return
				}
				result.ParseMarkdown(formatResult(response))
			})
		}()
	})
	if len(ready) == 0 {
		result.ParseMarkdown("_Start a ready Laya instance first._")
	}
	box := container.NewVBox(widget.NewLabelWithStyle("Playground", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), selectInstance, body, question, typeSelect, criteria, run, result)
	d.page.Content = box
	d.page.Refresh()
}

func parseCriteria(text string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func formatResult(result api.SystemOneResponse) string {
	keys := make([]string, 0, len(result.Answers))
	for key := range result.Answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		answer := result.Answers[key]
		lines = append(lines, fmt.Sprintf("### %s\n- type: `%s`\n- value: `%s`\n- confidence: %.2f", key, answer.Type, answerValue(answer), answer.Confidence))
	}
	return strings.Join(lines, "\n\n")
}

func answerValue(answer api.Answer) string {
	if answer.Type == "choice" {
		return answer.Choice
	}
	if answer.Type == "noul" {
		return fmt.Sprintf("%.2f", answer.Noul)
	}
	return fmt.Sprint(answer.Score)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
