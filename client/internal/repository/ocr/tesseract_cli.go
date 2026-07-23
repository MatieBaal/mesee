package ocr

// tesseractCLI - адаптер OCR, вызывающий утилиту tesseract
type TesseractCLI struct {
	lang string
}

func NewTesseractCLI(lang string) *TesseractCLI {

	if lang == "" {
		lang = "eng+rus"
	}
	return &TesseractCLI{
		lang: lang,
	}
}
