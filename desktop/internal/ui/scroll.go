package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// wheelOverlay forwards wheel movement above an editor to the page. It has no
// tap or focus behavior, so the editor underneath remains directly editable.
type wheelOverlay struct {
	widget.BaseWidget
	page  *container.Scroll
	entry *widget.Entry
}

func newWheelOverlay(page *container.Scroll, entry *widget.Entry) *wheelOverlay {
	overlay := &wheelOverlay{page: page, entry: entry}
	overlay.ExtendBaseWidget(overlay)
	return overlay
}

func (o *wheelOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.NRGBA{A: 1}))
}

func (o *wheelOverlay) Scrolled(event *fyne.ScrollEvent) { o.page.Scrolled(event) }

func (o *wheelOverlay) Tapped(event *fyne.PointEvent) {
	fyne.CurrentApp().Driver().CanvasForObject(o.entry).Focus(o.entry)
	o.entry.Tapped(event)
}

func (o *wheelOverlay) TappedSecondary(event *fyne.PointEvent) { o.entry.TappedSecondary(event) }

func (d *Desktop) pageWheelEntry(entry *widget.Entry) fyne.CanvasObject {
	return container.NewMax(entry, newWheelOverlay(d.page, entry))
}
