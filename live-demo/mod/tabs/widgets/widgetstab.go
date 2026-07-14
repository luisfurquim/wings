//go:build js && wasm

package widgets

import (
	_ "embed"
	"syscall/js"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/dom"
	"github.com/luisfurquim/wings/field"
	"github.com/luisfurquim/wings/wtext"
	"github.com/luisfurquim/wings/wtextepub"
)

//go:embed widgetstab.i18n.html
var htmlContent string

const cssContent = `
.widgets-tab { display: flex; flex-direction: column; gap: 24px; padding: 8px 0; }
.widgets-tab section { display: flex; flex-direction: column; gap: 12px; }
.widgets-tab h3 {
  margin: 0 0 4px;
  font-size: 0.85rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--wings-text-muted, #666);
  border-bottom: 1px solid var(--wings-border, #e0e0e0);
  padding-bottom: 6px;
}
.widget-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
`

type WidgetsTab struct{}

func init() {
	// The demo editor uses the full toolbar: basic marks + font/alignment
	// (FontToolbar) + named styles (StyleToolbar) + the passive
	// char/letter/word counter (CounterToolbar); EPUB export (wtextepub)
	// lands in the side menu under the standard Export group and registers
	// its book-metadata settings page (same instance in Menu and Config).
	// StyleLibrary adds the other pair of menu entries: save this
	// document's named styles to a file, load them into another one.
	epubExport := wtextepub.Menu{Cfg: wtextepub.Config{
		Title:  "Biografia",
		Author: "wings live demo",
	}}
	wtext.RegisterProfile("full", wtext.Profile{
		Toolbar: []wtext.ToolbarPlugin{
			wtext.BasicToolbar{},
			wtext.FontToolbar{},
			wtext.StyleToolbar{},
			wtext.CounterToolbar{},
		},
		Menu:   []wtext.MenuPlugin{epubExport, wtext.StyleLibrary{DefaultName: "meus-estilos"}},
		Config: []wtext.ConfigPlugin{epubExport},
	})
	wings.Register(
		"widgets-tab",
		htmlContent,
		cssContent,
		func() wings.PranaMod { return &WidgetsTab{} },
	)
}

func (w *WidgetsTab) InitData() map[string]any {
	return map[string]any{
		// Typed fields bound with &value: w-input validates them on blur in Go.
		"demo_email": field.NewEmail("email-bad"),
		"demo_age":   field.NewInt(0, 120, "age-nan", "age-bad"),
		"form_sent":  false,
		// Rich-text editor content, stored/read as EPUB-flavored HTML.
		"demo_bio": field.NewText(),
	}
}

func (w *WidgetsTab) Render(obj *wings.PranaObj) {
	// The validation section is a real native <form>: w-input is
	// form-associated, so an invalid field blocks submission by itself.
	// preventDefault keeps the demo page from navigating when it IS valid.
	forms := dom.Query(obj.Dom, "#validation-form")
	if len(forms) == 0 {
		return
	}
	dom.AddEvent(forms[0], "submit", func(_ js.Value, _ []js.Value) any {
		obj.This.Set("form_sent", true)
		return nil
	}, true, false)
	// The reset button (type="reset") clears the fields via each w-input's
	// formResetCallback; also clear the "sent" confirmation here.
	dom.AddEvent(forms[0], "reset", func(_ js.Value, _ []js.Value) any {
		obj.This.Set("form_sent", false)
		return nil
	}, false, false)
}
