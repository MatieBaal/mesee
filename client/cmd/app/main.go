package main

import (
	"context"
	"fmt"
	"client/internal/domain"
	"client/internal/repository/ocr"
	"client/internal/repository/translator"
	"client/internal/usecase"
)

func main() {
	fmt.Println("Запуск mesee...")
	screenshoter := ocr.NewScreenshoter()
	ocr := ocr.NewMockOCREngine()
	translatorService := translator.NewMockTranslator()

	appUseCase := usecase.NewTranslationUseCase(screenshoter, ocr, translatorService)
	testPoint := domain.Point{X: 300, Y: 200}
	ctx := context.Background()
	result, err := appUseCase.ProcessPoint(ctx, testPoint)
	if err != nil {
		fmt.Println("Ошибка!:", err)
		return
	}
	if result == nil {
		fmt.Println("Текст под курсором не найден!")
		return
	}
	fmt.Println("Все модули отработали!")
	fmt.Println("Оригинал:", result.OrigText)
	fmt.Println("Перевод:", result.Translated)

}
