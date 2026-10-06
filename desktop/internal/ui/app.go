package ui

import (
	"encoding/json"
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
	runtimepkg "github.com/1085924051/modelctl/desktop/internal/runtime"
)

type modelState struct {
	summary api.ModelSummary
	detail  api.ModelDetail
	variant string
	profile string
}

type Desktop struct {
	client         *api.Client
	app            fyne.App
	window         fyne.Window
	page           *container.Scroll
	status         *widget.Label
	models         []modelState
	instances      []api.Instance
	capabilities   []api.Capability
	runs           []api.Run
	selectedPage   string
	refreshMu      sync.Mutex
	playground     fyne.CanvasObject
	instanceSelect *widget.Select
	questionRows   []*questionEditor
	questionList   *fyne.Container
	result         *widget.RichText
	supervisor     *runtimepkg.Supervisor
	startupError   error
}

func New(client *api.Client) *Desktop {
	return NewWithSupervisor(client, nil, nil)
}

func NewWithSupervisor(client *api.Client, supervisor *runtimepkg.Supervisor, startupError error) *Desktop {
	application := app.NewWithID("com.modelctl.desktop")
	application.Settings().SetTheme(newTheme())
	desktop := &Desktop{client: client, app: application, selectedPage: "models", supervisor: supervisor, startupError: startupError}
	desktop.window = application.NewWindow("Modelctl")
	desktop.window.Resize(fyne.NewSize(1120, 760))
	desktop.page = container.NewVScroll(container.NewVBox())
	desktop.window.SetContent(desktop.layout())
	desktop.renderModels()
	return desktop
}

func (d *Desktop) Run() {
	if d.startupError != nil {
		d.status.SetText("Runtime unavailable: " + d.startupError.Error())
	} else {
		d.refresh()
	}
	d.window.ShowAndRun()
	if d.supervisor != nil {
		_ = d.supervisor.Close()
	}
}

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
		widget.NewButton("Capabilities", func() { d.showPage("capabilities") }),
		widget.NewButton("History", func() { d.showPage("history") }),
		widget.NewButton("Settings", func() { d.showPage("settings") }),
		layout.NewSpacer(),
		widget.NewLabel(d.client.BaseURL()),
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
	case "capabilities":
		d.renderCapabilities()
	case "history":
		d.renderHistory()
	case "settings":
		d.renderSettings()
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
			fyne.Do(func() { d.status.SetText(formatConnectionError(d.client.BaseURL(), err)) })
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
		capabilities, capabilityErr := d.client.Capabilities()
		if capabilityErr != nil {
			capabilities.Items = nil
		}
		runs, runsErr := d.client.Runs()
		if runsErr != nil {
			runs.Items = nil
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
			d.models = mergeModelStates(d.models, states)
			d.instances = instances.Items
			d.capabilities = capabilities.Items
			d.runs = runs.Items
			d.status.SetText(fmt.Sprintf("Connected · %d model(s) · %d ready instance(s)", len(states), len(readyInstances(instances.Items))))
			d.updateInstanceSelect()
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
	if len(d.capabilities) == 0 {
		box.Add(widget.NewCard("Start with a business capability", "Three steps from installation to integration", container.NewVBox(
			widget.NewLabel("1  Download model · verify the checkpoint"),
			widget.NewLabel("2  Load & run · choose CPU, MPS, or CUDA when available"),
			widget.NewLabel("3  Capabilities · publish a stable contract for your application"),
			widget.NewButton("Open Capabilities", func() { d.showPage("capabilities") }),
		)))
	}
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
	runningLabel := widget.NewLabel("")
	updateRunning := func(variant string) {
		runningLabel.SetText("")
		for _, instance := range readyInstances(d.instances) {
			if instance.Model.ID == model.summary.ID && instance.Model.Variant == variant {
				runningLabel.SetText(fmt.Sprintf("Running on %s · %s", strings.ToUpper(instance.Device), instance.ID))
				return
			}
		}
	}
	variantSelect := widget.NewSelect(variantIDs, func(selected string) {
		model.variant = selected
		stateLabel.SetText(modelStateText(model.summary, selected))
		updateRunning(selected)
	})
	variantSelect.SetSelected(model.variant)
	updateRunning(model.variant)
	profileSelect := widget.NewSelect(supportedProfileIDs(model.detail.Preflight.Profiles), func(selected string) { model.profile = selected })
	profileSelect.SetSelected(model.profile)
	progress := widget.NewLabel("")
	var pull *widget.Button
	var run *widget.Button
	cancel := widget.NewButton("Cancel", nil)
	cancel.Hide()
	pull = widget.NewButton("Download model", func() { d.pull(model, progress, stateLabel, pull, run, cancel) })
	run = widget.NewButton("Load & run", func() { d.run(model, progress, stateLabel, pull, run) })
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("%s · v%s", model.summary.ID, model.summary.Version)),
		stateLabel,
		runningLabel,
		container.NewGridWithColumns(2, widget.NewLabel("Variant"), variantSelect, widget.NewLabel("Device"), profileSelect),
		container.NewHBox(pull, run, cancel, progress),
	)
	name := model.detail.DisplayName
	if name == "" {
		name = model.summary.ID
	}
	return widget.NewCard(name, "Structured decision", content)
}

func modelStateText(model api.ModelSummary, variant string) string {
	if installedVariant(model, variant) {
		return "Downloaded · ready to load"
	}
	return "Not downloaded · Download verifies the checkpoint"
}

func (d *Desktop) pull(model *modelState, progress, stateLabel *widget.Label, pull, run, cancel *widget.Button) {
	request := api.PullRequest{ModelID: model.summary.ID, Version: model.summary.Version, Variant: model.variant}
	pull.Disable()
	run.Disable()
	cancel.Show()
	cancel.Enable()
	var taskID string
	cancel.OnTapped = func() {
		if taskID == "" {
			return
		}
		cancel.Disable()
		go func() {
			_, err := d.client.CancelTask(taskID)
			fyne.Do(func() {
				if err != nil {
					progress.SetText(err.Error())
					cancel.Enable()
					return
				}
				progress.SetText("Cancelling download...")
			})
		}()
	}
	go func() {
		defer fyne.Do(func() { pull.Enable(); run.Enable(); cancel.Hide(); cancel.Enable() })
		result, err := d.client.Pull(request)
		if err != nil {
			fyne.Do(func() { progress.SetText(err.Error()) })
			return
		}
		taskID = result.TaskID
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
				fyne.Do(func() { stateLabel.SetText("Downloaded · ready to load"); progress.SetText("Download verified") })
				d.refresh()
				return
			}
			if task.Status == "failed" || task.Status == "cancelled" {
				message := "Download " + task.Status
				if task.Error != nil && task.Error.Message != "" {
					message = task.Error.Message
				}
				fyne.Do(func() { progress.SetText(message) })
				return
			}
			time.Sleep(750 * time.Millisecond)
		}
	}()
}

func (d *Desktop) run(model *modelState, progress, stateLabel *widget.Label, pull, run *widget.Button) {
	summary, variant, profile := model.summary, model.variant, model.profile
	pull.Disable()
	run.Disable()
	go func() {
		if !installedVariant(summary, variant) {
			fyne.Do(func() { progress.SetText("Pulling before run…") })
			if err := d.pullAndWait(summary, variant, func(text string) { fyne.Do(func() { progress.SetText(text) }) }); err != nil {
				fyne.Do(func() { progress.SetText(err.Error()); pull.Enable(); run.Enable() })
				return
			}
		}
		instance, err := d.client.StartInstance(api.StartInstanceRequest{ModelID: summary.ID, Version: summary.Version, Variant: variant, Profile: profile, Default: true})
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
		if err == nil {
			d.refresh()
		}
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
			if task.Error != nil && task.Error.Message != "" {
				return fmt.Errorf("download %s: %s", task.Status, task.Error.Message)
			}
			return fmt.Errorf("download %s", task.Status)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func (d *Desktop) renderRunning() {
	active := activeInstances(d.instances)
	box := container.NewVBox(widget.NewLabelWithStyle("Running", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(active) == 0 {
		box.Add(widget.NewLabel("No running instances"))
	}
	for _, instance := range active {
		instance := instance
		status := widget.NewLabel("")
		stop := widget.NewButton("Stop", nil)
		stop.OnTapped = func() {
			stop.Disable()
			status.SetText("Stopping…")
			go func() {
				_, err := d.client.StopInstance(instance.ID)
				fyne.Do(func() {
					if err != nil {
						status.SetText(err.Error())
						stop.Enable()
						return
					}
					status.SetText("Stopped")
				})
				if err == nil {
					d.refresh()
				}
			}()
		}
		line := fmt.Sprintf("%s · %s · %s", instance.Status, strings.ToUpper(instance.Device), instance.ID)
		box.Add(widget.NewCard(instance.Model.Variant, instance.Model.ID, container.NewBorder(nil, nil, widget.NewLabel(line), stop, status)))
	}
	d.page.Content = box
	d.page.Refresh()
}

func (d *Desktop) renderPlayground() {
	if d.playground == nil {
		d.buildPlayground()
	}
	d.updateInstanceSelect()
	d.page.Content = d.playground
	d.page.Refresh()
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
		lines = append(lines, fmt.Sprintf("### %s\n**%s** · %s", key, answerValue(answer), answer.Type))
		if answer.Confidence > 0 {
			lines = append(lines, fmt.Sprintf("Confidence: %.0f%%", answer.Confidence*100))
		}
		if len(answer.Probabilities) > 0 {
			labels := make([]string, 0, len(answer.Probabilities))
			for label := range answer.Probabilities {
				labels = append(labels, label)
			}
			sort.Slice(labels, func(i, j int) bool { return answer.Probabilities[labels[i]] > answer.Probabilities[labels[j]] })
			for _, label := range labels {
				name := label
				if legend := answer.Legend[label]; legend != "" {
					name = legend
				}
				lines = append(lines, fmt.Sprintf("- %s: %.1f%%", name, answer.Probabilities[label]*100))
			}
		}
	}
	return strings.Join(lines, "\n\n")
}

func formatRawResult(result api.SystemOneResponse) string {
	raw, _ := json.MarshalIndent(result, "", "  ")
	return string(raw)
}

func answerValue(answer api.Answer) string {
	if answer.Value != nil {
		return fmt.Sprint(answer.Value)
	}
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
