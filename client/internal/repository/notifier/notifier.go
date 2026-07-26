package notifier

import (
	"context"
	"fmt"
	"os/exec"
)

type DesktopNotifier struct {
	appName string
}

func NewDesktopNotifier(appName string) *DesktopNotifier {
	if appName == "" {
		appName = "Mesee"
	}
	return &DesktopNotifier{
		appName: appName,
	}
}

func (n *DesktopNotifier) Notify(ctx context.Context, title, message string) error {
	// Вызываем утилиту notify-send с указанием имени приложения и таймаута (3000 мс = 3 сек)
	cmd := exec.CommandContext(ctx, "notify-send",
		"-a", n.appName,
		"-t", "3000",
		"-h", "string:x-canonical-private-synchronous:mesee-notify", // Заменяет предыдущее уведомление новым, а не спамит стопкой
		title,
		message,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}
