# Mesee

**Mesee** is a lightweight, high-performance screen-capture daemon written in C, coupled with an extensible Go desktop application. It features a modular, dynamic backend architecture (`.so`) designed to work seamlessly across both **Wayland** (Hyprland, Sway, etc.) and **X11** environments with precise multi-monitor coordination.

---

## Architecture Overview
```
           +-------------------+
           |  Go Client (UI)   |
           +---------+---------+
                     |
                     | IPC / Shared Memory
                     v
           +-------------------+
           |   C Daemon Core   |
           +----+---------+----+
                |         |
       +--------+         +---------------+
       |                                  |
       v (dlopen)                         v (dlopen)
+-----------------------+         +-----------------------+
|  Wayland Backend .so  |         |    X11 Backend .so    |
| (wlr-screencopy, xdg) |         |     (Xlib, XGetImage) |
+-----------------------+         +-----------------------+
```

* **Core Daemon (`daemon/`):** C-based service handling dynamic backend loading, coordinate calculations, and shared memory management.
* **Wayland Backend:** Utilizes `wlr-screencopy-unstable-v1` and `xdg-output-unstable-v1` for accurate region capturing and fractional scaling support.
* **X11 Backend:** Uses `Xlib` on the Root Window to capture exact virtual desktop coordinates cleanly without software cursor overlays.
* **Go Client (`client/`):** Graphical/CLI interface that coordinates screen capture, OCR tasks (via Tesseract), and translation pipelines.

---

## Build Prerequisites

To build Mesee manually from source, ensure you have the following dependencies installed:

### C Daemon Dependencies
* `gcc`
* `make`
* `wayland-client` & `wayland-scanner` (for Wayland backend)
* `libX11` (for X11 backend)

### Go Client Dependencies
* `go` (v1.20+)
* `tesseract` (for OCR capabilities)

---

## Building from Source

1. **Clone the repository:**
   ```bash
   git clone [https://github.com/MatieBaal/mesee.git](https://github.com/MatieBaal/mesee.git)
   cd mesee
Build the C Daemon & Backends:

Bash
make
This compiles the mesee_daemon binary alongside libmesee_backend_wayland.so and libmesee_backend_x11.so into the build/ directory.

Build the Go Client:

Bash
cd client
go build -o bin/mesee-client ./cmd/app/main.go
Arch Linux (AUR Installation)
Mesee is split into granular AUR packages so users on X11 do not need to install heavy Wayland dependencies, and vice versa.

For Wayland Users (Hyprland, Sway, GNOME, KDE)
Bash
yay -S mesee-common mesee-backend-wayland
For X11 Users
Bash
yay -S mesee-common mesee-backend-x11