package ocr

import (
	"context"
	"image"
	"client/internal/domain"
)

type MockOCR struct{}

func NewMockOCREngine() *MockOCR {
	return &MockOCR{}
}

func (m *MockOCR) RecognizeText(ctx context.Context, img image.Image) (*domain.RecognizedText, error) {
	return &domain.RecognizedText{
		Text: "Hello world",
		Bounds: domain.BoundingBox{
			X:      0,
			Y:      0,
			Width:  img.Bounds().Dx(),
			Height: img.Bounds().Dy(),
		},
	}, nil
}
