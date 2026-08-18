package usecase

import (
	"context"
	"fmt"
	"image"
	"strings"
	"time"

	"client/internal/domain"
	ocrrepo "client/internal/repository/ocr"
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
	const captureWidth = 300
	const captureHeight = 100
	const captureAttempts = 3
	const captureDelay = 300 * time.Millisecond

	localCursor := image.Pt(captureWidth/2, captureHeight/2)
	var candidates []ocrrepo.CandidateResult

	for attempt := 0; attempt < captureAttempts; attempt++ {
		img, err := tuc.capturer.CaptureArea(ctx, point, captureWidth, captureHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to capture screen:%w", err)
		}
		if img == nil {
			continue
		}

		words, err := tuc.ocr.Recognize(ctx, img)
		if err != nil {
			return nil, fmt.Errorf("failed to recognize text:%w", err)
		}
		if len(words) == 0 {
			if attempt < captureAttempts-1 {
				time.Sleep(captureDelay)
			}
			continue
		}

		var targetWord *domain.RecognizedWord
		for _, word := range words {
			if localCursor.In(word.Bounds) {
				w := word
				targetWord = &w
				break
			}
		}
		if targetWord == nil {
			if attempt < captureAttempts-1 {
				time.Sleep(captureDelay)
			}
			continue
		}

		var dictionary []string
		for _, candidate := range candidates {
			if text := strings.TrimSpace(candidate.Text); text != "" {
				dictionary = append(dictionary, text)
			}
		}
		for _, word := range words {
			if text := strings.TrimSpace(word.Text); text != "" {
				dictionary = append(dictionary, text)
			}
		}

		matcher := ocrrepo.NewFuzzyMatcher(false)
		matcher.LoadWords(dictionary)
		corrected, fuzzyScore := matcher.FindBestMatch(targetWord.Text)
		if corrected == "" || (corrected == targetWord.Text && fuzzyScore == 0) {
			corrected = targetWord.Text
			fuzzyScore = 100
		}

		candidates = append(candidates, ocrrepo.CandidateResult{
			Text:       corrected,
			Score:      fuzzyScore,
			Confidence: targetWord.Confidence,
			Count:      1,
			Bounds:     targetWord.Bounds,
		})

		if attempt < captureAttempts-1 {
			time.Sleep(captureDelay)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	best := ocrrepo.ChooseBestCandidate(candidates)
	chosenText := best.Text
	if chosenText == "" {
		chosenText = candidates[0].Text
	}

	translated, err := tuc.translator.Translate(ctx, chosenText)
	if err != nil {
		return nil, fmt.Errorf("translation failed:%w", err)
	}

	notifyTitle := fmt.Sprintf("Перевод %s", chosenText)
	if err := tuc.notifier.Notify(ctx, notifyTitle, translated); err != nil {
		fmt.Printf("[WARN] Failed to send system notification: %v\n", err)
	}

	resultBounds := domain.BoundingBox{
		X:      best.Bounds.Min.X,
		Y:      best.Bounds.Min.Y,
		Width:  best.Bounds.Dx(),
		Height: best.Bounds.Dy(),
	}

	return &domain.TranslationResult{
		OrigText:   chosenText,
		Translated: translated,
		Bounds:     resultBounds,
	}, nil
}
