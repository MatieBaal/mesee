package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// UI управляет графическим окном приложения
type UI struct {
	app        fyne.App
	window     fyne.Window
	origInput  *widget.Entry
	transInput *widget.Entry
}

// NewUI создаёт и настраивает главное окно
func NewUI() *UI {
	a := app.New()
	w := a.NewWindow("mesee — Screen Translator")

	// Поле для распознанного текста (многострочное)
	origInput := widget.NewMultiLineEntry()
	origInput.SetPlaceHolder("Здесь появится распознанный текст (OCR)...")

	// Поле для перевода (многострочное)
	transInput := widget.NewMultiLineEntry()
	transInput.SetPlaceHolder("Здесь появится перевод...")

	// Формируем красивую разметку с заголовками
	content := container.NewVBox(
		widget.NewLabelWithStyle("Оригинал (OCR):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		origInput,
		widget.NewLabelWithStyle("Перевод:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		transInput,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(400, 300)) // Начальный размер окна

	return &UI{
		app:        a,
		window:     w,
		origInput:  origInput,
		transInput: transInput,
	}
}

// ShowAndRun запускает главный цикл событий GUI (блокирующий вызов)
func (ui *UI) ShowAndRun() {
	ui.window.ShowAndRun()
}

// UpdateText обновляет содержимое полей из любого горутины
func (ui *UI) UpdateText(orig, translated string) {
	// В Fyne изменения UI из фоновых горутин желательно делать безопасно
	ui.origInput.SetText(orig)
	ui.transInput.SetText(translated)
}
