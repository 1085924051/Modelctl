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
		widget.NewLabelWithStyle("Run history", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Replayable local records keep the capability version, model instance, input and structured answer together."),
	)
	if len(d.runs) == 0 {
		box.Add(widget.NewLabel("No runs yet. Test a published capability first."))
	} else {
		for _, run := range d.runs {
			run := run
			raw, _ := json.MarshalIndent(run.Response, "", "  ")
			status := widget.NewLabel("")
			rerun := widget.NewButton("Run again", nil)
			rerun.OnTapped = func() {
				rerun.Disable()
				status.SetText("Running...")
				go func() {
					result, err := d.client.InvokeCapability(run.CapabilityID, api.CapabilityInvokeRequest{Input: run.Input}, run.CapabilityVersion)
					fyne.Do(func() {
						rerun.Enable()
						if err != nil { status.SetText(err.Error()); return }
						status.SetText("Replayed as " + result.RunID)
						d.refresh()
					})
				}()
			}
			box.Add(widget.NewCard(
				run.CapabilityID+" · "+run.CapabilityVersion,
				run.CreatedAt,
				container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Run %s · instance %s", run.ID, run.InstanceID)),
				widget.NewLabel("Input: "+runInputText(run.Input)),
				container.NewHBox(rerun, status),
				widget.NewAccordion(widget.NewAccordionItem("Raw result", widget.NewLabel(string(raw)))),
				),
			))
		}
	}
	d.page.Content = box
	d.page.Refresh()
}

func runInputText(input map[string]any) string {
	if text, ok := input["text"].(string); ok { return text }
	payload, _ := json.Marshal(input)
	return string(payload)
}
