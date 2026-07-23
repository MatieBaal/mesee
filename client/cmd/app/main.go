package main

import (
	"context"
	"fmt"
	"mesee/internal/domain"
	"mesee/internal/repository/ocr"
	"mesee/internal/repository/translator"
	"mesee/internal/usecase"
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
