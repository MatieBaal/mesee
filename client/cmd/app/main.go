package main

import (
	"context"
	"fmt"

	"client/internal/repository/daemon"
	"client/internal/repository/ocr"
	"client/internal/repository/translator"
	"client/internal/usecase"
)

const socketPath = "/tmp/mesee.sock"

func main() {
	fmt.Println("Запуск mesee...")

	daemonClient := daemon.NewDaemonClient(socketPath)

	cursorPoint, err := daemonClient.GetCursorPos()
	if err != nil {
		fmt.Printf("Ошибка подключения к демону: %v\n", err)
		return
	}
	fmt.Printf("Курсор найден в точке: X=%d, Y=%d\n", cursorPoint.X, cursorPoint.Y)

	// 3. Делаем снимок через демона (запрашиваем рамку 10x10 вокруг курсора)
	outputFile := "build/tests/capture.jpg"
	_, err = daemonClient.CaptureArea(10, 10, 10, 10, outputFile)
	if err != nil {
		fmt.Printf("Ошибка захвата области: %v\n", err)
		return
	}
	fmt.Printf("Снимок экрана сохранен в: %s\n", outputFile)

	screenshoter := ocr.NewScreenshoter()
	ocrEngine := ocr.NewMockOCREngine()
	translatorService := translator.NewMockTranslator()

	appUseCase := usecase.NewTranslationUseCase(screenshoter, ocrEngine, translatorService)

	ctx := context.Background()
	result, err := appUseCase.ProcessPoint(ctx, cursorPoint)
	if err != nil {
		fmt.Println("Ошибка!:", err)
		return
	}
	if result == nil {
		fmt.Println("Текст под курсором не найден!")
		return
	}

	fmt.Println("\n--- Все модули отработали! ---")
	fmt.Println("Оригинал:", result.OrigText)
	fmt.Println("Перевод:", result.Translated)
}
