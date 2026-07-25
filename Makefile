CC ?= gcc
CFLAGS ?= -Wall -Wextra -O2 -g
INCLUDES = -Idaemon -Idaemon/backend -Idaemon/backend/x11 -Idaemon/backend/wayland

BUILD_DIR = build
WAYLAND_PROTOCOLS_DIR = daemon/protocols
WAYLAND_GEN_DIR = $(BUILD_DIR)/protocols

# Итоговый бинарник демона
DAEMON_BIN = $(BUILD_DIR)/mesee_daemon
DAEMON_SRCS = daemon/main.c
DAEMON_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(DAEMON_SRCS))

# Динамическая библиотека X11
X11_SO = $(BUILD_DIR)/libmesee_backend_x11.so
X11_SRCS = $(wildcard daemon/backend/x11/*.c)
X11_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(X11_SRCS))
X11_LIBS = -lX11

# Сгенерированные протоколы Wayland
WAYLAND_GEN_HEADERS = $(WAYLAND_GEN_DIR)/wlr-screencopy-unstable-v1-client-protocol.h \
                      $(WAYLAND_GEN_DIR)/xdg-output-unstable-v1-client-protocol.h
WAYLAND_PROTO_OBJS  = $(WAYLAND_GEN_DIR)/wlr-screencopy-unstable-v1-protocol.o \
                      $(WAYLAND_GEN_DIR)/xdg-output-unstable-v1-protocol.o

# Динамическая библиотека Wayland
WAYLAND_SO = $(BUILD_DIR)/libmesee_backend_wayland.so
WAYLAND_SRCS = $(wildcard daemon/backend/wayland/*.c)
WAYLAND_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(WAYLAND_SRCS)) $(WAYLAND_PROTO_OBJS)
WAYLAND_LIBS = -lwayland-client

.PHONY: all clean

all: $(DAEMON_BIN) $(X11_SO) $(WAYLAND_SO)

# --- Генерация кода Wayland из XML ---
$(WAYLAND_GEN_DIR)/%-protocol.c: $(WAYLAND_PROTOCOLS_DIR)/%.xml
	@mkdir -p $(dir $@)
	wayland-scanner private-code < $< > $@

$(WAYLAND_GEN_DIR)/%-client-protocol.h: $(WAYLAND_PROTOCOLS_DIR)/%.xml
	@mkdir -p $(dir $@)
	wayland-scanner client-header < $< > $@

$(WAYLAND_GEN_DIR)/%-protocol.o: $(WAYLAND_GEN_DIR)/%-protocol.c $(WAYLAND_GEN_DIR)/%-client-protocol.h
	$(CC) $(CFLAGS) -fPIC -c $< -o $@

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
		echo " [!] X11 source files not found, skip building $(X11_SO)"; \
	fi

# --- Сборка Wayland бэкенда (.so) ---
$(WAYLAND_SO): $(WAYLAND_OBJS)
	@mkdir -p $(dir $@)
	@if [ -n "$(WAYLAND_OBJS)" ]; then \
		$(CC) -shared $(WAYLAND_OBJS) -o $@ $(WAYLAND_LIBS); \
		echo " -> Wayland бэкенд собран: $@"; \
	else \
		echo " [!] Wayland source files not found, skip building $(WAYLAND_SO)"; \
	fi

# --- Точные правила компиляции объектов ---

# 1. Бинарник демона (без -fPIC)
$(BUILD_DIR)/daemon/main.o: daemon/main.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) $(INCLUDES) -c $< -o $@

# 2. X11 Бэкенд (обязательно с -fPIC)
$(BUILD_DIR)/daemon/backend/x11/%.o: daemon/backend/x11/%.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -fPIC $(INCLUDES) -c $< -o $@

# 3. Wayland Бэкенд (с -fPIC, гарантированной генерацией всех протоколов)
$(BUILD_DIR)/daemon/backend/wayland/%.o: daemon/backend/wayland/%.c $(WAYLAND_GEN_HEADERS)
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -fPIC $(INCLUDES) -I$(WAYLAND_GEN_DIR) -c $< -o $@

clean:
	rm -rf $(BUILD_DIR)
	@echo " -> Dir $(BUILD_DIR) cleaned"