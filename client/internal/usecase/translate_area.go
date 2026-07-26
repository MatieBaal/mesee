package usecase

import (
	"context"
	"fmt"
	"image"

	"client/internal/domain"
)

type TranslationUseCase struct {
	capturer   domain.ScreenCapturer
	ocr        domain.OCREngine
	translator domain.TranslatorService
	notifier   domain.Notifier
}

// NewTranslationUseCase - основной usecase
func NewTranslationUseCase(
	c domain.ScreenCapturer,
	o domain.OCREngine,
	t domain.TranslatorService,
	n domain.Notifier,
) *TranslationUseCase {
	return &TranslationUseCase{
		capturer:   c,
		ocr:        o,
		translator: t,
		notifier:   n,
	}
}

func (tuc *TranslationUseCase) ProcessPoint(ctx context.Context, point domain.Point) (*domain.TranslationResult, error) {
	// Размеры окна захвата вокруг курсора
	const captureWidth = 300
	const captureHeight = 100

	// 1. Снимок экрана
	img, err := tuc.capturer.CaptureArea(ctx, point, captureWidth, captureHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen:%w", err)
	}
	if img == nil {
		return nil, nil
	}

	// 2. Распознавание текста
	words, err := tuc.ocr.Recognize(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("failed to recognize text:%w", err)
	}
	if len(words) == 0 {
		return nil, nil // Текста в области захвата нет
	}

	// 3. Вычисляем локальные координаты курсора внутри скриншота.
	localCursor := image.Pt(captureWidth/2, captureHeight/2)

	// 4. Ищем слово, в рамку которого попадает локальный курсор
	var targetWord *domain.RecognizedWord
	for _, word := range words {
		if localCursor.In(word.Bounds) {
			w := word
			targetWord = &w
			break
		}
	}

	if targetWord == nil {
		return nil, nil // Курсор наведен на пустое место
	}

	// 5. Перевод найденного слова
	translated, err := tuc.translator.Translate(ctx, targetWord.Text)
	if err != nil {
		return nil, fmt.Errorf("translation failed:%w", err)
	}

	notifyTitle := fmt.Sprintf("Перевод %s", targetWord.Text)
	if err := tuc.notifier.Notify(ctx, notifyTitle, translated); err != nil {
		// Логируем ошибку уведомления, но не ломаем основной поток
		fmt.Printf("[WARN] Failed to send system notification: %v\n", err)
	}

	// 6. Конвертируем image.Rectangle обратно в domain.BoundingBox
	return &domain.TranslationResult{
		OrigText:   targetWord.Text,
		Translated: translated,
		Bounds: domain.BoundingBox{
			X:      targetWord.Bounds.Min.X,
			Y:      targetWord.Bounds.Min.Y,
			Width:  targetWord.Bounds.Dx(),
			Height: targetWord.Bounds.Dy(),
		},
	}, nil
}
