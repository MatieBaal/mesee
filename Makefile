CC ?= gcc
CFLAGS ?= -Wall -Wextra -O2 -g
INCLUDES = -Idaemon -Idaemon/backend -Idaemon/backend/x11 -Idaemon/backend/wayland

BUILD_DIR = build

# Итоговый бинарник демона
DAEMON_BIN = $(BUILD_DIR)/mesee_daemon
DAEMON_SRCS = daemon/main.c
DAEMON_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(DAEMON_SRCS))

# Динамическая библиотека X11
X11_SO = $(BUILD_DIR)/libmesee_backend_x11.so
X11_SRCS = $(wildcard daemon/backend/x11/*.c)
X11_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(X11_SRCS))
X11_LIBS = -lX11

# Динамическая библиотека Wayland
WAYLAND_SO = $(BUILD_DIR)/libmesee_backend_wayland.so
WAYLAND_SRCS = $(wildcard daemon/backend/wayland/*.c)
WAYLAND_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(WAYLAND_SRCS))
WAYLAND_LIBS = -lwayland-client

.PHONY: all clean

all: $(DAEMON_BIN) $(X11_SO) $(WAYLAND_SO)

# --- Сборка основного демона ---
$(DAEMON_BIN): $(DAEMON_OBJS)
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) $(DAEMON_OBJS) -o $@ -ldl
	@echo " -> Демон успешно собран: $@"

# --- Сборка X11 бэкенда (.so) ---
$(X11_SO): $(X11_OBJS)
	@mkdir -p $(dir $@)
	@if [ -n "$(X11_OBJS)" ]; then \
		$(CC) -shared $(X11_OBJS) -o $@ $(X11_LIBS); \
		echo " -> X11 бэкенд собран: $@"; \
	else \
		echo " [!] X11 source files not founded, skip building $(X11_SO)"; \
	fi

# --- Сборка Wayland бэкенда (.so) ---
$(WAYLAND_SO): $(WAYLAND_OBJS)
	@mkdir -p $(dir $@)
	@if [ -n "$(WAYLAND_OBJS)" ]; then \
		$(CC) -shared $(WAYLAND_OBJS) -o $@ $(WAYLAND_LIBS); \
		echo " -> Wayland бэкенд собран: $@"; \
	else \
		echo " [!] Wayland source files not founded, skip building $(WAYLAND_SO)"; \
	fi

# --- Компиляция .c файлов демона ---
$(BUILD_DIR)/daemon/%.o: daemon/%.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) $(INCLUDES) -c $< -o $@

$(BUILD_DIR)/daemon/backend/%.o: daemon/backend/%.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -fPIC $(INCLUDES) -c $< -o $@

clean:
	rm -rf $(BUILD_DIR)
	@echo " -> Dir $(BUILD_DIR) cleaned"