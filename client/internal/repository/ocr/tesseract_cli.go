package ocr

import (
	"bytes"
	"client/internal/domain"
	"context"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
)

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

func (tcli *TesseractCLI) RecognizeText(ctx context.Context, img image.Image) (*domain.RecognizedText, error) {
	if img == nil {
		return nil, nil
	}

	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image:%w", err)
	}
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "-l", tcli.lang)
	cmd.Stdin = &imgBuf
	var outBuf, errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Faled to run tesseract CLI:%w", err)
	}
	recognizedText := strings.TrimSpace(outBuf.String())
	if recognizedText == "" {
		return nil, nil
	}

	return &domain.RecognizedText{
		Text: recognizedText,
		Bounds: domain.BoundingBox{
			X:      0,
			Y:      0,
			Width:  img.Bounds().Dx(),
			Height: img.Bounds().Dy(),
		},
	}, nil
}
