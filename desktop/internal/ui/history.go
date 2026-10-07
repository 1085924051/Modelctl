package ui

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

func (d *Desktop) renderHistory() {
	box := container.NewVBox(
		widget.NewLabelWithStyle(d.t("Run history", "调用记录"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(d.t("Replayable local records keep the capability version, model instance, input and structured answer together.", "本地记录保留能力版本、模型实例、输入和结构化结果，便于复查。")),
	)
	if len(d.runs) == 0 {
		box.Add(widget.NewLabel(d.t("No runs yet. Test a published capability first.", "还没有调用记录。请先测试已发布的业务能力。")))
	} else {
		for _, run := range d.runs {
			run := run
			raw, _ := json.MarshalIndent(run.Response, "", "  ")
			status := widget.NewLabel("")
			rerun := widget.NewButton(d.t("Run again", "再次运行"), nil)
			rerun.OnTapped = func() {
				rerun.Disable()
				status.SetText(d.t("Running...", "正在运行…"))
				go func() {
					result, err := d.client.InvokeCapability(run.CapabilityID, api.CapabilityInvokeRequest{Input: run.Input, Metadata: run.Metadata}, run.CapabilityVersion)
					fyne.Do(func() {
						rerun.Enable()
						if err != nil {
							status.SetText(err.Error())
							return
						}
						status.SetText(d.t("Replayed as ", "新运行记录：") + result.RunID)
						d.refresh()
					})
				}()
			}
			box.Add(widget.NewCard(
				run.CapabilityID+" · "+run.CapabilityVersion,
				run.CreatedAt,
				container.NewVBox(
					widget.NewLabel(fmt.Sprintf(d.t("Run %s · instance %s", "记录 %s · 实例 %s"), run.ID, run.InstanceID)),
					widget.NewLabel(d.t("Input: ", "输入：")+runInputText(run.Input)),
					metadataLabel(run.Metadata),
					container.NewHBox(rerun, status),
					widget.NewRichTextFromMarkdown(formatResult(run.Response)),
					widget.NewAccordion(widget.NewAccordionItem(d.t("Raw result", "原始结果"), widget.NewLabel(string(raw)))),
				),
			))
		}
	}
	d.page.Content = box
	d.page.Refresh()
}

func runInputText(input map[string]any) string {
	if text, ok := input["text"].(string); ok {
		return text
	}
	payload, _ := json.Marshal(input)
	return string(payload)
}

func metadataLabel(metadata map[string]any) fyne.CanvasObject {
	if len(metadata) == 0 {
		return widget.NewLabel("")
	}
	payload, _ := json.Marshal(metadata)
	return widget.NewLabel("Metadata: " + string(payload))
}
