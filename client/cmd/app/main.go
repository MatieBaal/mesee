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
	wayland_hotkey "client/internal/repository/hotkey" // Алиас для Wayland-обработчика
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

	// 1. Инициализируем адаптеры
	daemonClient := daemon.NewDaemonClient(socketPath)
	ocrEngine := ocr.NewTesseractCLI("")
	translatorService := translator.NewGoogleTranslator("")
	notifierMesee := notifier.NewDesktopNotifier("")

	// 2. Сборка UseCase
	appUseCase := usecase.NewTranslationUseCase(daemonClient, ocrEngine, translatorService, notifierMesee)

	backendType, err := daemonClient.GetBackendType(context.Background())
	if err != nil {
		log.Fatalf("Ошибка определения типа бекенда: %v", err)
	}

	if backendType == 0x01 {
		// --- ВЕТКА X11 / WINDOWS / MACOS ---
		fmt.Println("Обнаружен классический оконный сервер (X11/Win/Mac).")

		hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyS)
		_ = hk.Unregister()
		err = hk.Register()
		if err != nil {
			log.Fatalf("Ошибка регистрации хоткея: %v", err)
		}
		defer hk.Unregister() // <- Скобки обязательны!

		fmt.Println("Готово! Нажмите [Ctrl + Shift + S] для перевода.")
		fmt.Println("Для выхода нажмите Ctrl+C в терминале.")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case <-hk.Keydown():
				fmt.Println("\n[Сработал хоткей X11] Начинаем процесс перевода...")

				cursorPoint, err := daemonClient.GetCursorPos()
				if err != nil {
					fmt.Printf("Ошибка получения координат: %v\n", err)
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

	} else if backendType == 0x02 {
		// --- ВЕТКА WAYLAND ---
		fmt.Println("Обнаружен Wayland. Инициализация D-Bus XDG Portal...")

		waylandListener, err := wayland_hotkey.NewWaylandListener()
		if err != nil {
			log.Fatalf("Ошибка запуска Wayland listener: %v", err)
		}
		defer waylandListener.Close() // <- Скобки обязательны!

		fmt.Println("Готово! Нажмите настроенный Wayland-хоткей для перевода.")
		fmt.Println("Для выхода нажмите Ctrl+C в терминале.")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // <- Скобки обязательны!

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\nЗавершение работы mesee...")
			cancel()
		}()

		err = waylandListener.Listen(ctx, func() {
			fmt.Println("\n[Сработал хоткей Wayland] Начинаем процесс перевода...")

			cursorPoint, err := daemonClient.GetCursorPos()
			if err != nil {
				fmt.Printf("Ошибка получения координат: %v\n", err)
				return
			}

			result, err := appUseCase.ProcessPoint(ctx, cursorPoint)
			if err != nil {
				fmt.Println("Ошибка!:", err)
				return
			}
			if result == nil {
				fmt.Println("Текст под курсором не найден!")
				return
			}

			fmt.Println("--- Результат ---")
			fmt.Println("Оригинал:", result.OrigText)
			fmt.Println("Перевод:", result.Translated)
		})

		if err != nil && err != context.Canceled {
			log.Fatalf("Работа слушателя прервана: %v", err)
		}
	} else {
		log.Fatalf("Неизвестный тип бекенда: %v", backendType)
	}
}
