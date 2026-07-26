package hotkey

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// WaylandListener отвечает за получение событий хоткея из D-Bus
type WaylandListener struct {
	conn *dbus.Conn
}

func NewWaylandListener() (*WaylandListener, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}
	return &WaylandListener{conn: conn}, nil
}

// Close закрывает соединение с системной шиной
func (w *WaylandListener) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}

// Listen блокирует поток и ждёт сигналов, пока не будет отменён ctx
func (w *WaylandListener) Listen(ctx context.Context, onActivate func()) error {
	// Добавляем правило фильтрации для подписки на сигнал
	rule := "type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated'"
	call := w.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
	if call.Err != nil {
		return fmt.Errorf("failed to add dbus match: %w", call.Err)
	}

	// Создаём буферизованный канал для сигналов
	c := make(chan *dbus.Signal, 10)
	w.conn.Signal(c)

	fmt.Println("[Wayland] Ожидание сигналов от XDG Portal...")

	// Бесконечный цикл обработки
	for {
		select {
		case <-ctx.Done(): // Поступил сигнал остановки программы
			w.conn.RemoveSignal(c)
			return ctx.Err()
		case sig := <-c:
			if sig.Name == "org.freedesktop.portal.GlobalShortcuts.Activated" {
				// sig.Body обычно содержит [session_handle, shortcut_id, timestamp, options]
				shortcutID := "unknown"
				if len(sig.Body) > 1 {
					shortcutID = fmt.Sprintf("%v", sig.Body[1])
				}

				fmt.Printf("[Wayland] Хоткей сработал! ID: %s\n", shortcutID)

				// Вызываем callback, который запустит основную логику
				onActivate()
			}
		}
	}
}
