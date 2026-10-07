package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/1085924051/modelctl/desktop/internal/api"
)

func TestPlaygroundPageScrollsOverForm(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	d := &Desktop{app: application, client: api.NewClient(""), language: "zh", selectedPage: "playground"}
	d.window = application.NewWindow("Modelctl")
	d.page = container.NewVScroll(container.NewVBox())
	d.window.SetContent(d.layout())
	d.window.Resize(fyne.NewSize(800, 400))
	d.showPage("playground")
	for i := 0; i < 8; i++ {
		d.addQuestion()
	}
	d.window.Show()
	pos := application.Driver().AbsolutePositionForObject(d.playgroundBody).Add(fyne.NewPos(10, 10))
	test.TapCanvas(d.window.Canvas(), pos)
	if d.window.Canvas().Focused() != d.playgroundBody {
		t.Fatal("wheel overlay blocked editing the material field")
	}
	test.Type(d.playgroundBody, "sample")
	if d.playgroundBody.Text != "sample" {
		t.Fatalf("material field did not accept typing: %q", d.playgroundBody.Text)
	}
	test.Scroll(d.window.Canvas(), pos, 0, -100)
	if d.page.Offset.Y <= 0 {
		t.Fatalf("page did not scroll over form: offset=%v", d.page.Offset)
	}
}

func TestLanguageSwitchPreservesPlaygroundDraft(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	d := &Desktop{app: application, client: api.NewClient(""), language: "zh", selectedPage: "playground"}
	d.window = application.NewWindow("Modelctl")
	d.page = container.NewVScroll(container.NewVBox())
	d.window.SetContent(d.layout())
	d.showPage("playground")
	d.playgroundBody.SetText("客户要求退款")
	d.questionRows[0].instructions.SetText("是否应退款？")
	var language *widget.Select
	var find func(fyne.CanvasObject)
	find = func(object fyne.CanvasObject) {
		if selectWidget, ok := object.(*widget.Select); ok && len(selectWidget.Options) == 2 && selectWidget.Options[1] == "English" {
			language = selectWidget
		}
		if group, ok := object.(*fyne.Container); ok {
			for _, child := range group.Objects {
				find(child)
			}
		}
	}
	find(d.window.Content())
	if language == nil {
		t.Fatal("language switch missing")
	}
	language.SetSelected("English")
	if d.language != "en" || d.playgroundBody.Text != "客户要求退款" || d.questionRows[0].instructions.Text != "是否应退款？" {
		t.Fatal("switching language discarded the in-progress form")
	}
	if d.navButtons["playground"].Importance != widget.HighImportance {
		t.Fatal("active navigation state was lost")
	}
}
