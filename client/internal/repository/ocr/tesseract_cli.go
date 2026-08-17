package ocr

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"image"
	"image/png"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"client/internal/domain"
)

// TesseractCLI - адаптер OCR, вызывающий утилиту tesseract
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

// Recognize распознаёт изображение и возвращает срез слов с их координатами
func (tcli *TesseractCLI) Recognize(ctx context.Context, img image.Image) ([]domain.RecognizedWord, error) {
	if img == nil {
		return nil, nil
	}

	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "-l", tcli.lang, "tsv")
	cmd.Stdin = &imgBuf

	var outBuf, errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run tesseract CLI: %w (stderr: %s)", err, errBuf.String())
	}

	return parseTesseractTSV(&outBuf)
}

// parseTesseractTSV читает табличный вывод Tesseract и собирает []domain.RecognizedWord
func parseTesseractTSV(r io.Reader) ([]domain.RecognizedWord, error) {
	reader := csv.NewReader(r)
	reader.Comma = '\t'      // Разделитель табуляции
	reader.LazyQuotes = true // Игнорируем кавычки в тексте

	// Пропускаем заголовок таблицы
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read tsv header: %w", err)
	}

	var words []domain.RecognizedWord

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Пропускаем повреждённые строки
		}

		if len(record) < 12 {
			continue
		}

		level := record[0]
		text := strings.TrimSpace(record[11])

		// level "5" отвечает за конкретное слово
		if level != "5" || text == "" {
			continue
		}

		left, _ := strconv.Atoi(record[6])
		top, _ := strconv.Atoi(record[7])
		width, _ := strconv.Atoi(record[8])
		height, _ := strconv.Atoi(record[9])
		conf, _ := strconv.Atoi(record[10])

		if conf < 0 {
			continue
		}

		// Было:
		// Bounds: domain.BoundingBox{
		// 	X:      left,
		// 	Y:      top,
		// 	Width:  width,
		// 	Height: height,
		// },

		// Стало:
		words = append(words, domain.RecognizedWord{
			Text:       text,
			Bounds:     image.Rect(left, top, left+width, top+height),
			Confidence: conf,
		})
	}

	return words, nil
}
