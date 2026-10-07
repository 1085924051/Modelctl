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
		status.SetText(d.t("A bearer token is configured for this session.", "此会话已配置访问令牌。"))
	}

	apply := widget.NewButton(d.t("Apply connection", "应用连接设置"), func() {
		d.client.SetBaseURL(baseURL.Text)
		value := strings.TrimSpace(token.Text)
		if value == "" {
			status.SetText(d.t("URL applied. Leave token blank for local mode or to keep the current token.", "地址已应用。令牌留空可使用本机模式或保留原令牌。"))
			return
		}
		d.client.SetToken(value)
		status.SetText(d.t("URL and token applied for this session. Business services should use their own secret storage.", "地址和令牌已应用于此会话。业务系统应使用自己的密钥存储。"))
	})
	box := container.NewVBox(
		widget.NewLabelWithStyle(d.t("Connection", "服务连接"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(d.t("Use loopback mode for local development. Enterprise administrators can enter a remote service token here.", "本机开发可直接连接本地服务；企业管理员可填写远程服务令牌。")),
		widget.NewLabel("Modelctl URL"), baseURL,
		widget.NewLabel(d.t("API token", "API 令牌")), token,
		widget.NewLabel(d.t("The token stays in memory and is never included in copied REST or SDK examples.", "令牌仅保存在内存中，不会进入复制的 REST 或 SDK 示例。")),
		container.NewHBox(apply, status),
	)
	d.page.Content = container.NewVBox(widget.NewCard(d.t("Service connection", "服务连接"), d.t("Local or enterprise endpoint", "本机或企业服务地址"), box))
	d.page.Refresh()
}
