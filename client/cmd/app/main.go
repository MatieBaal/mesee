package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"

	"client/internal/repository/daemon"
	"client/internal/repository/ocr"
	"client/internal/repository/translator"
	"client/internal/usecase"
)

const socketPath = "/tmp/mesee.sock"

func main() {
	// 1. Инициализация главного потока через mainthread
	mainthread.Init(run)
}

func run() {
	fmt.Println("Запуск mesee в фоновом режиме...")

	daemonClient := daemon.NewDaemonClient(socketPath)
	screenshoter := ocr.NewScreenshoter()
	ocrEngine := ocr.NewTesseractCLI("")
	translatorService := translator.NewGoogleTranslator("")

	appUseCase := usecase.NewTranslationUseCase(screenshoter, ocrEngine, translatorService)

	// 2. Регистрируем комбинацию: Ctrl + Alt + S
	// В пакете hotkey модификаторы передаются срезом []hotkey.Modifier
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyS)

	err := hk.Register()
	if err != nil {
		log.Fatalf("Ошибка регистрации хоткея: %v", err)
	}
	defer hk.Unregister()

	fmt.Println("Готово! Нажмите [Ctrl + Alt + S] для перевода.")
	fmt.Println("Для выхода нажмите Ctrl+C в терминале.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-hk.Keydown():
			fmt.Println("\n[Сработал хоткей] Запрос к C-демону...")

			cursorPoint, err := daemonClient.GetCursorPos()
			if err != nil {
				fmt.Printf("Ошибка подключения к демону: %v\n", err)
				continue
			}

			outputFile := "build/tests/capture.jpg"
			_, err = daemonClient.CaptureArea(10, 10, 10, 10, outputFile)
			if err != nil {
				fmt.Printf("Ошибка захвата области: %v\n", err)
				continue
			}

			ctx := context.Background()
			result, err := appUseCase.ProcessPoint(ctx, cursorPoint)
			if err != nil {
				fmt.Println("Ошибка!:", err)
				continue
			}
			if result == nil {
				fmt.Println("Текст под курсором не найден!")
				continue
			}

			fmt.Println("--- Результат ---")
			fmt.Println("Оригинал:", result.OrigText)
			fmt.Println("Перевод:", result.Translated)

		case <-sigChan:
			fmt.Println("\nЗавершение работы mesee...")
			return
		}
	}
}
