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
	client            *api.Client
	app               fyne.App
	window            fyne.Window
	page              *container.Scroll
	status            *widget.Label
	models            []modelState
	instances         []api.Instance
	capabilities      []api.Capability
	runs              []api.Run
	selectedPage      string
	refreshMu         sync.Mutex
	playground        fyne.CanvasObject
	instanceSelect    *widget.Select
	questionRows      []*questionEditor
	questionList      *fyne.Container
	result            *widget.RichText
	supervisor        *runtimepkg.Supervisor
	startupError      error
	connectionError   string
	modelsLoaded      bool
	language          string
	navButtons        map[string]*widget.Button
	playgroundBody    *widget.Entry
	playgroundRaw     *widget.Entry
	playgroundMode    *widget.RadioGroup
	pendingPlayground *playgroundState
}

func New(client *api.Client) *Desktop {
	return NewWithSupervisor(client, nil, nil)
}

func NewWithSupervisor(client *api.Client, supervisor *runtimepkg.Supervisor, startupError error) *Desktop {
	application := app.NewWithID("com.modelctl.desktop")
	application.Settings().SetTheme(newTheme())
	desktop := &Desktop{client: client, app: application, selectedPage: "models", supervisor: supervisor, startupError: startupError, language: application.Preferences().StringWithFallback("language", "zh")}
	desktop.window = application.NewWindow("Modelctl")
	desktop.window.Resize(fyne.NewSize(1120, 760))
	desktop.page = container.NewVScroll(container.NewVBox())
	desktop.window.SetContent(desktop.layout())
	desktop.showPage("models")
	return desktop
}

func (d *Desktop) Run() {
	if d.startupError != nil {
		d.connectionError = "Runtime unavailable: " + d.startupError.Error()
		d.status.SetText(d.connectionError)
		d.renderModels()
	} else {
		d.refresh()
	}
	d.window.ShowAndRun()
	if d.supervisor != nil {
		_ = d.supervisor.Close()
	}
}

func (d *Desktop) layout() fyne.CanvasObject {
	statusText := d.t("Connecting…", "正在连接…")
	if d.connectionError != "" {
		statusText = d.connectionError
	} else if d.modelsLoaded {
		statusText = fmt.Sprintf(d.t("Connected · %d model(s) · %d ready instance(s)", "已连接 · %d 个模型 · %d 个就绪实例"), len(d.models), len(readyInstances(d.instances)))
	}
	d.status = widget.NewLabel(statusText)
	refresh := widget.NewButton(d.t("Refresh", "刷新"), d.refresh)
	language := widget.NewSelect([]string{"中文", "English"}, func(value string) {
		selected := "zh"
		if value == "English" {
			selected = "en"
		}
		if selected == d.language {
			return
		}
		d.language = selected
		d.app.Preferences().SetString("language", selected)
		if d.playgroundBody != nil {
			state := &playgroundState{body: d.playgroundBody.Text, raw: d.playgroundRaw.Text, jsonMode: d.playgroundMode.Selected == "JSON"}
			for _, row := range d.questionRows {
				state.questions = append(state.questions, row.draft())
			}
			d.pendingPlayground = state
		}
		d.playground = nil
		d.window.SetContent(d.layout())
		d.showPage(d.selectedPage)
	})
	if d.language == "en" {
		language.SetSelected("English")
	} else {
		language.SetSelected("中文")
	}
	header := container.NewBorder(nil, nil, widget.NewLabel("MODELCTL"), container.NewHBox(language, refresh), d.status)
	d.navButtons = make(map[string]*widget.Button)
	navButton := func(page, en, zh string) *widget.Button {
		button := widget.NewButton(d.t(en, zh), func() { d.showPage(page) })
		d.navButtons[page] = button
		return button
	}
	nav := container.NewVBox(
		widget.NewLabelWithStyle(d.t("LOCAL MODEL STUDIO", "本地模型工作台"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		navButton("models", "Models", "模型下载"),
		navButton("running", "Running", "运行中的模型"),
		navButton("playground", "Playground", "试用模型"),
		navButton("capabilities", "Capabilities", "业务能力"),
		navButton("integration", "Integration", "接入应用"),
		navButton("history", "History", "调用记录"),
		navButton("settings", "Settings", "设置"),
		layout.NewSpacer(),
		widget.NewLabel(d.client.BaseURL()),
	)
	return container.NewBorder(container.NewPadded(header), nil, container.NewPadded(nav), nil, container.NewPadded(d.page))
}

func (d *Desktop) showPage(page string) {
	if d.selectedPage != page {
		d.page.Offset = fyne.NewPos(0, 0)
	}
	d.selectedPage = page
	for name, button := range d.navButtons {
		if name == page {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		button.Refresh()
	}
	switch page {
	case "running":
		d.renderRunning()
	case "playground":
		d.renderPlayground()
	case "capabilities":
		d.renderCapabilities()
	case "integration":
		d.renderIntegration()
	case "history":
		d.renderHistory()
	case "settings":
		d.renderSettings()
	default:
		d.renderModels()
	}
}

func (d *Desktop) t(en, zh string) string {
	if d.language == "en" {
		return en
	}
	return zh
}

func (d *Desktop) refresh() {
	if !d.refreshMu.TryLock() {
		return
	}
	go func() {
		defer d.refreshMu.Unlock()
		if err := d.client.Health(); err != nil {
			fyne.Do(func() { d.showConnectionError(formatConnectionError(d.client.BaseURL(), err)) })
			return
		}
		models, err := d.client.Models()
		if err != nil {
			fyne.Do(func() { d.showConnectionError("Models unavailable: " + err.Error()) })
			return
		}
		instances, err := d.client.Instances()
		if err != nil {
			fyne.Do(func() { d.showConnectionError("Instances unavailable: " + err.Error()) })
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
		var modelDetailError error
		for _, summary := range models.Items {
			detail, detailErr := d.client.ModelDetail(summary.ID)
			if detailErr != nil {
				modelDetailError = detailErr
				continue
			}
			states = append(states, modelState{summary: summary, detail: detail, variant: chooseVariant(detail.Variants, ""), profile: firstSupported(detail.Preflight.Profiles)})
		}
		if len(models.Items) > 0 && len(states) == 0 {
			fyne.Do(func() { d.showConnectionError("Model details unavailable: " + modelDetailError.Error()) })
			return
		}
		fyne.Do(func() {
			d.connectionError = ""
			d.modelsLoaded = true
			d.models = mergeModelStates(d.models, states)
			d.instances = instances.Items
			d.capabilities = capabilities.Items
			d.runs = runs.Items
			d.status.SetText(fmt.Sprintf(d.t("Connected · %d model(s) · %d ready instance(s)", "已连接 · %d 个模型 · %d 个就绪实例"), len(states), len(readyInstances(instances.Items))))
			d.updateInstanceSelect()
			d.showPage(d.selectedPage)
		})
	}()
}

func (d *Desktop) showConnectionError(message string) {
	d.connectionError = message
	d.status.SetText(message)
	if d.selectedPage == "models" {
		d.renderModels()
	}
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
	box := container.NewVBox(widget.NewLabelWithStyle(d.t("Download and run models", "下载并加载模型"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if d.connectionError != "" {
		message := widget.NewLabel(d.connectionError)
		message.Wrapping = fyne.TextWrapWord
		box.Add(widget.NewCard(d.t("Model service unavailable", "模型服务未就绪"), d.t("The download list needs the model service", "下载列表需要连接模型服务"), container.NewVBox(
			message,
			widget.NewButton(d.t("Retry connection", "重试连接"), d.refresh),
		)))
	}
	if len(d.models) == 0 {
		if d.connectionError == "" && !d.modelsLoaded {
			box.Add(widget.NewLabel(d.t("Connecting to the model service…", "正在连接模型服务…")))
		} else if d.connectionError == "" {
			box.Add(widget.NewLabel(d.t("No models in the catalog. Check the service catalog configuration.", "模型目录为空，请检查服务端目录配置。")))
		}
	}
	for index := range d.models {
		box.Add(d.modelCard(index))
	}
	if len(d.models) > 0 && len(d.capabilities) == 0 {
		box.Add(widget.NewCard(d.t("Next steps", "接下来怎么用"), d.t("Start with the model card above", "从上方模型卡片开始"), container.NewVBox(
			widget.NewLabel(d.t("1. Select a variant and download its weights.", "1. 选择模型版本并下载权重。")),
			widget.NewLabel(d.t("2. Load the model and try it in Playground.", "2. 加载模型，再到“试用模型”页体验。")),
			widget.NewLabel(d.t("3. Publish a capability for your application to call.", "3. 发布业务能力，供你的应用调用。")),
			widget.NewButton(d.t("Open Capabilities", "创建业务能力"), func() { d.showPage("capabilities") }),
		)))
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
	stateLabel := widget.NewLabel(d.modelStateLabel(model.summary, model.variant))
	runningLabel := widget.NewLabel("")
	updateRunning := func(variant string) {
		runningLabel.SetText("")
		for _, instance := range readyInstances(d.instances) {
			if instance.Model.ID == model.summary.ID && instance.Model.Variant == variant {
				runningLabel.SetText(fmt.Sprintf(d.t("Running on %s · %s", "运行于 %s · %s"), strings.ToUpper(instance.Device), instance.ID))
				return
			}
		}
	}
	variantSelect := widget.NewSelect(variantIDs, func(selected string) {
		model.variant = selected
		stateLabel.SetText(d.modelStateLabel(model.summary, selected))
		updateRunning(selected)
	})
	variantSelect.SetSelected(model.variant)
	updateRunning(model.variant)
	profileSelect := widget.NewSelect(supportedProfileIDs(model.detail.Preflight.Profiles), func(selected string) { model.profile = selected })
	profileSelect.SetSelected(model.profile)
	progress := widget.NewLabel("")
	var pull *widget.Button
	var run *widget.Button
	cancel := widget.NewButton(d.t("Cancel", "取消"), nil)
	cancel.Hide()
	pull = widget.NewButton(d.t("Download model", "下载模型"), func() { d.pull(model, progress, stateLabel, pull, run, cancel) })
	run = widget.NewButton(d.t("Load & run", "加载并运行"), func() { d.run(model, progress, stateLabel, pull, run) })
	pull.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("%s · v%s", model.summary.ID, model.summary.Version)),
		stateLabel,
		runningLabel,
		container.NewGridWithColumns(2, widget.NewLabel(d.t("Variant", "模型版本")), variantSelect, widget.NewLabel(d.t("Device", "运行设备")), profileSelect),
		container.NewHBox(pull, run, cancel, progress),
	)
	name := model.detail.DisplayName
	if name == "" {
		name = model.summary.ID
	}
	return widget.NewCard(name, d.t("Structured decision", "将资料转成结构化判断"), content)
}

func (d *Desktop) modelStateLabel(model api.ModelSummary, variant string) string {
	if installedVariant(model, variant) {
		return d.t("Downloaded · ready to load", "已下载，可加载")
	}
	return d.t("Not downloaded · download to continue", "尚未下载，先下载权重")
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
				progress.SetText(fmt.Sprintf(d.t("Downloading %d%%", "下载中 %d%%"), progressPercent(task.Progress.BytesDone, task.Progress.BytesTotal)))
			})
			if task.Status == "succeeded" {
				fyne.Do(func() {
					stateLabel.SetText(d.t("Downloaded · ready to load", "已下载，可加载"))
					progress.SetText(d.t("Download verified", "下载已校验"))
				})
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
			fyne.Do(func() { progress.SetText(d.t("Pulling before run…", "运行前正在下载…")) })
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
			progress.SetText(d.t("Ready", "已就绪"))
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
		report(fmt.Sprintf(d.t("Downloading %d%%", "下载中 %d%%"), progressPercent(task.Progress.BytesDone, task.Progress.BytesTotal)))
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
	box := container.NewVBox(widget.NewLabelWithStyle(d.t("Running models", "运行中的模型"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(active) == 0 {
		box.Add(widget.NewLabel(d.t("No running instances. Load a model from Models first.", "还没有运行中的模型。请先到“模型下载”页加载。")))
	}
	for _, instance := range active {
		instance := instance
		status := widget.NewLabel("")
		stop := widget.NewButton(d.t("Stop", "停止"), nil)
		stop.OnTapped = func() {
			stop.Disable()
			status.SetText(d.t("Stopping…", "正在停止…"))
			go func() {
				_, err := d.client.StopInstance(instance.ID)
				fyne.Do(func() {
					if err != nil {
						status.SetText(err.Error())
						stop.Enable()
						return
					}
					status.SetText(d.t("Stopped", "已停止"))
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
