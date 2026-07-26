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
	"client/internal/repository/notifier"
	"client/internal/repository/ocr"
	"client/internal/repository/translator"
	"client/internal/usecase"
)

const socketPath = "/tmp/mesee.sock"

func main() {
	mainthread.Init(run)
}

func run() {
	fmt.Println("Запуск mesee в фоновом режиме...")

	// 1. Инициализируем адаптеры (репозитории)
	daemonClient := daemon.NewDaemonClient(socketPath)
	ocrEngine := ocr.NewTesseractCLI("")
	translatorService := translator.NewGoogleTranslator("")
	notifierMesee := notifier.NewDesktopNotifier("")

	// 2. Сборка UseCase (внедрение зависимостей)
	// daemonClient передается как реализация интерфейса ScreenCapturer!
	appUseCase := usecase.NewTranslationUseCase(daemonClient, ocrEngine, translatorService, notifierMesee)

	backendType, err := daemonClient.GetBackendType(context.Background())
	if err != nil {
		log.Fatalf("Backend type difinition error:%w", err)
	}

	if backendType == 0x01 {

		// 3. Регистрация хоткея
		hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyS)
		_ = hk.Unregister() // Очистка зомби-регистраций
		err = hk.Register()
		if err != nil {
			log.Fatalf("Ошибка регистрации хоткея: %v", err)
		}
		defer hk.Unregister()

		fmt.Println("Готово! Нажмите [Ctrl + Shift + S] для перевода.")
		fmt.Println("Для выхода нажмите Ctrl+C в терминале.")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case <-hk.Keydown():
				fmt.Println("\n[Сработал хоткей] Начинаем процесс перевода...")

				// Контроллер отвечает только за старт бизнес-операции
				// и передачу ей начальных параметров (например, точки координат).
				cursorPoint, err := daemonClient.GetCursorPos()
				if err != nil {
					fmt.Printf("Ошибка получения координат: %v\n", err)
					continue
				}

				// Бизнес-логика (захват, распознавание, перевод) инкапсулирована внутри UseCase
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

}
