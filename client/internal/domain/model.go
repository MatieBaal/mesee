package domain

// Entities
// Положение курсора
type Point struct {
	X int
	Y int
}

// Положение и размер текста
type BoundingBox struct {
	X, Y, Width, Height int
}

// Результат работы OCR
type RecognizedText struct {
	Text   string
	Bounds BoundingBox
}

// Переведённый текст
type TranslationResult struct {
	OrigText   string
	Translated string
	Bounds     BoundingBox
}
