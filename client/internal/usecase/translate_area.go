package usecase

import (
	"context"
	"fmt"
	"mesee/internal/domain"
)

type TranslationUseCase struct {
	capturer   domain.ScreenCapturer
	ocr        domain.OCREngine
	translator domain.TranslatorService
}

// основной usecase
func NewTranslationUseCase(
	c domain.ScreenCapturer,
	o domain.OCREngine,
	t domain.TranslatorService,
) *TranslationUseCase {
	return &TranslationUseCase{
		capturer:   c,
		ocr:        o,
		translator: t,
	}
}

func (tuc *TranslationUseCase) ProcessPoint(ctx context.Context, point domain.Point) (*domain.TranslationResult, error) {
	// Снимок экрана
	img, err := tuc.capturer.CaptureArea(ctx, point, 300, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen:%w", err)
	}
	// Распознавание текста
	OcrResult, err := tuc.ocr.RecognizeText(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("Falied to recognize text:%w", err)
	}
	if OcrResult == nil || OcrResult.Text == "" {
		return nil, nil // Текста под курсором нет
	}

	translated, err := tuc.translator.Translate(ctx, OcrResult.Text, "RU")
	if err != nil {
		return nil, fmt.Errorf("translation failed:%w", err)
	}
	return &domain.TranslationResult{
		OrigText:   OcrResult.Text,
		Translated: translated,
		Bounds:     OcrResult.Bounds,
	}, nil
}
