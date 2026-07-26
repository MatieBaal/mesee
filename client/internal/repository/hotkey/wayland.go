package hotkey

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const hardcodedID = "wl_mesee_hotkey" // Захардкоженный ID хоткея для Wayland

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

func (w *WaylandListener) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}

func (w *WaylandListener) Listen(ctx context.Context, onActivate func()) error {
	// Слушаем НАШ кастомный сигнал вместо сложного XDG Portal
	rule := "type='signal',interface='com.mesee.hotkey',member='Activated'"
	call := w.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
	if call.Err != nil {
		return fmt.Errorf("failed to add dbus match: %w", call.Err)
	}

	c := make(chan *dbus.Signal, 10)
	w.conn.Signal(c)

	fmt.Printf("[Wayland] Ожидание сигнала '%s' по кастомной шине...\n", hardcodedID)

	for {
		select {
		case <-ctx.Done():
			w.conn.RemoveSignal(c)
			return ctx.Err()
		case sig := <-c:
			if sig.Name == "com.mesee.hotkey.Activated" {
				// Проверяем, что нам передали строку (наш ID)
				if len(sig.Body) > 0 {
					shortcutID := fmt.Sprintf("%v", sig.Body[0])

					if shortcutID == hardcodedID {
						fmt.Printf("[Wayland] Захардкоженный хоткей '%s' сработал! Запускаем логику.\n", shortcutID)
						onActivate()
					}
				}
			}
		}
	}
}
