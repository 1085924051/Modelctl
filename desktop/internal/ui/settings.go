package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (d *Desktop) renderSettings() {
	baseURL := widget.NewEntry()
	baseURL.SetText(d.client.BaseURL())
	token := widget.NewPasswordEntry()
	status := widget.NewLabel("")
	if d.client.HasToken() {
		status.SetText("A bearer token is configured for this session.")
	}

	apply := widget.NewButton("Apply connection", func() {
		d.client.SetBaseURL(baseURL.Text)
		value := strings.TrimSpace(token.Text)
		if value == "" {
			status.SetText("URL applied. Leave token blank for local loopback mode or to keep the current token.")
			return
		}
		d.client.SetToken(value)
		status.SetText("URL and token applied for this session. Business services should use their own secret storage.")
	})
	box := container.NewVBox(
		widget.NewLabelWithStyle("Connection", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Use loopback mode for local development. Enterprise administrators can enter a remote service token here."),
		widget.NewLabel("Modelctl URL"), baseURL,
		widget.NewLabel("API token"), token,
		widget.NewLabel("The token stays in memory and is never included in copied REST or SDK examples."),
		container.NewHBox(apply, status),
	)
	d.page.Content = container.NewVScroll(container.NewVBox(widget.NewCard("Service connection", "Local or enterprise endpoint", box)))
	d.page.Refresh()
}
