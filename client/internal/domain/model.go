package domain

import (
	"context"
	"image"
)

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

// RecognizedWord описывает отдельное слово и его границы на скриншоте
type RecognizedWord struct {
	Text       string          // Само слово (например, "Hello")
	Bounds     image.Rectangle // Прямоугольник с координатами (Left, Top, Right, Bottom)
	Confidence int             // Уверенность Tesseract в % (от 0 до 100)
}

type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}
