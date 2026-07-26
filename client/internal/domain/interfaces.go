package domain

import (
	"context"
	"image"
)

// Ports

// Порт для снятия фрагмента экрана
type ScreenCapturer interface {
	CaptureArea(ctx context.Context, center Point, width int, height int) (image.Image, error)
}

// Порт для распознавания текста
type OCREngine interface {
	Recognize(ctx context.Context, img image.Image) ([]RecognizedWord, error)
}

// Порт для перевода текста
type TranslatorService interface {
	Translate(ctx context.Context, text string) (string, error)
}
