# Makefile — PDFMaster Hybrid (Go + C++ Engine)
# ══════════════════════════════════════════════════════════════════════
# Usage:
#   make              Build everything
#   make install      Install to ~/.local/bin
#   make clean        Remove build artefacts
# ══════════════════════════════════════════════════════════════════════

# ── Paths ─────────────────────────────────────────────────────────────
ROOT        := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ENGINE_DIR  := $(ROOT)engine
GO_DIR      := $(ROOT)go
BUILD_DIR   := $(ROOT)build
DIST_DIR    := $(ROOT)dist
PREFIX      ?= $(HOME)/.local

# ── Detect platform ───────────────────────────────────────────────────
UNAME := $(shell uname -s)
ifeq ($(UNAME),Linux)
  PLATFORM   := linux
  EXE_EXT    :=
  STATIC_EXT := .a
else ifeq ($(UNAME),Darwin)
  PLATFORM   := darwin
  EXE_EXT    :=
  STATIC_EXT := .a
else
  PLATFORM   := windows
  EXE_EXT    := .exe
  STATIC_EXT := .lib
endif

# ── Build type ────────────────────────────────────────────────────────
BUILD_TYPE  ?= Release
BUILD_LOWER := $(shell echo $(BUILD_TYPE) | tr '[:upper:]' '[:lower:]')

# ── Engine paths ──────────────────────────────────────────────────────
ENGINE_BUILD := $(BUILD_DIR)/engine-$(BUILD_LOWER)
ENGINE_LIB   := $(ENGINE_BUILD)/lib/libpdfengine$(STATIC_EXT)
ENGINE_INC   := $(ENGINE_DIR)/include

# ── Output binary ─────────────────────────────────────────────────────
BINARY_NAME := pdfmaster$(EXE_EXT)
BINARY_OUT  := $(BUILD_DIR)/$(BINARY_NAME)

# ── Parallelism ───────────────────────────────────────────────────────
NPROC := $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# ── CMake generator ───────────────────────────────────────────────────
CMAKE_GEN := $(shell which ninja >/dev/null 2>&1 && echo "Ninja" || echo "Unix Makefiles")

# ── Dependency detection ──────────────────────────────────────────────
MUPDF_FOUND := $(shell pkg-config --exists mupdf   2>/dev/null && echo "ON" || echo "OFF")
QPDF_FOUND  := $(shell pkg-config --exists libqpdf 2>/dev/null && echo "ON" || echo "OFF")

# ── CGo flags ─────────────────────────────────────────────────────────
ifeq ($(UNAME),Linux)
  MUPDF_STATIC_LIBS := $(shell pkg-config --static --libs mupdf 2>/dev/null || \
    echo "-lmupdf -lmupdf-third -lmujs -lgumbo -lopenjp2 -ljbig2dec -ljpeg -lz -lm -lfreetype -lbz2 -lpng16 -lbrotlidec -lbrotlicommon")
  QPDF_LIBS := $(shell pkg-config --libs libqpdf 2>/dev/null || echo "-lqpdf")

  export CGO_CFLAGS  = -I$(ENGINE_INC)
  export CGO_LDFLAGS = -L$(ENGINE_BUILD)/lib -lpdfengine \
                       $(MUPDF_STATIC_LIBS) \
                       $(QPDF_LIBS) \
                       -lharfbuzz -lstdc++ -lm -lpthread -ldl

else ifeq ($(UNAME),Darwin)
  export CGO_CFLAGS  = -I$(ENGINE_INC)
  export CGO_LDFLAGS = -L$(ENGINE_BUILD)/lib -lpdfengine \
                       $(shell pkg-config --static --libs mupdf 2>/dev/null || echo "-lmupdf") \
                       $(shell pkg-config --libs libqpdf 2>/dev/null || echo "-lqpdf") \
                       -lc++ -lm -lpthread -ldl
else
  export CGO_CFLAGS  = -I$(ENGINE_INC)
  export CGO_LDFLAGS = -L$(ENGINE_BUILD)/lib -lpdfengine -lmupdf -lqpdf -lstdc++ -lm
endif

# ═══════════════════════════════════════════════════════════════════════
.PHONY: all
all: engine go-deps go
	@echo ""
	@echo "  ✓  PDFMaster built successfully"
	@echo "     Binary : $(BINARY_OUT)"
	@echo "     Run    : $(BINARY_OUT) --help"
	@echo ""

# ═══════════════════════════════════════════════════════════════════════
.PHONY: engine
engine: $(ENGINE_LIB)

$(ENGINE_LIB): $(ENGINE_DIR)/CMakeLists.txt \
               $(shell find $(ENGINE_DIR)/src -name '*.cpp' -o -name '*.h' 2>/dev/null) \
               $(ENGINE_DIR)/include/pdfengine.h
	@echo "  [engine] Configuring ($(BUILD_TYPE), $(CMAKE_GEN))..."
	@mkdir -p $(ENGINE_BUILD)
	@cmake -B $(ENGINE_BUILD) -S $(ENGINE_DIR) \
	    -G "$(CMAKE_GEN)" \
	    -DCMAKE_BUILD_TYPE=$(BUILD_TYPE) \
	    -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
	    -DENGINE_USE_SYSTEM_MUPDF=$(MUPDF_FOUND) \
	    -DENGINE_USE_SYSTEM_QPDF=$(QPDF_FOUND) \
	    -DENGINE_ENABLE_LTO=$(shell [ "$(BUILD_TYPE)" = "Release" ] && echo ON || echo OFF) \
	    $(if $(MUPDF_ROOT),-DMUPDF_ROOT=$(MUPDF_ROOT),) \
	    $(if $(QPDF_ROOT),-DQPDF_ROOT=$(QPDF_ROOT),) \
	    -DCMAKE_MODULE_PATH="$(ROOT)cmake" \
	    --log-level=WARNING
	@echo "  [engine] Building with $(NPROC) jobs..."
	@cmake --build $(ENGINE_BUILD) --parallel $(NPROC)
	@echo "  [engine] Library: $(ENGINE_LIB)"

# ═══════════════════════════════════════════════════════════════════════
.PHONY: go-deps
go-deps:
	@if [ ! -f "$(GO_DIR)/go.sum" ]; then \
	    echo "  [go] Fetching Go dependencies..."; \
	    cd $(GO_DIR) && go mod tidy; \
	fi

# ═══════════════════════════════════════════════════════════════════════
.PHONY: go
go: $(BINARY_OUT)

$(BINARY_OUT): $(ENGINE_LIB) $(shell find $(GO_DIR) -name '*.go' 2>/dev/null)
	@echo "  [go] Building binary..."
	@mkdir -p $(BUILD_DIR)
	@cd $(GO_DIR) && \
	    CGO_ENABLED=1 \
	    CGO_CFLAGS="$(CGO_CFLAGS)" \
	    CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	    go build \
	        -ldflags="-s -w \
	            -X main.buildVersion=1.0.0 \
	            -X main.buildDate=$(shell date -u +%Y-%m-%d) \
	            -X main.buildCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo dev)" \
	        -o $(BINARY_OUT) \
	        ./cmd/pdfmaster
	@echo "  [go] Binary: $(BINARY_OUT)"

# ═══════════════════════════════════════════════════════════════════════
.PHONY: test
test: engine go-deps
	@cd $(GO_DIR) && CGO_ENABLED=1 CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go test -v ./...

# ═══════════════════════════════════════════════════════════════════════
.PHONY: install
install: all
	@echo "  [install] Installing to $(PREFIX)/bin/..."
	@mkdir -p $(PREFIX)/bin
	@cp $(BINARY_OUT) $(PREFIX)/bin/$(BINARY_NAME)
	@chmod +x $(PREFIX)/bin/$(BINARY_NAME)
	@echo "  ✓  Installed: $(PREFIX)/bin/$(BINARY_NAME)"
ifeq ($(PLATFORM),linux)
	@mkdir -p $(PREFIX)/share/applications
	@cp $(ROOT)engine/resources/linux/pdfmaster.desktop \
	    $(PREFIX)/share/applications/ 2>/dev/null || true
endif
	@echo ""
	@echo "  Ensure PATH includes $(PREFIX)/bin:"
	@echo "    echo 'export PATH=\$$PATH:$(PREFIX)/bin' >> ~/.bashrc && source ~/.bashrc"

# ═══════════════════════════════════════════════════════════════════════
.PHONY: clean distclean dist
clean:
	@rm -rf $(BUILD_DIR)
	@echo "  [clean] Done"

distclean: clean
	@rm -rf $(DIST_DIR)
	@echo "  [distclean] Done"

dist: all
	@mkdir -p $(DIST_DIR)
	@cp $(BINARY_OUT) $(DIST_DIR)/
	@TARBALL="$(DIST_DIR)/pdfmaster-1.0.0-$(PLATFORM)-amd64.tar.gz"; \
	    tar -czf $$TARBALL -C $(DIST_DIR) $(BINARY_NAME) && \
	    echo "  [dist] $$TARBALL"

# ═══════════════════════════════════════════════════════════════════════
# DEPENDENCY INSTALL HELPERS
# ═══════════════════════════════════════════════════════════════════════
.PHONY: deps-ubuntu deps-arch deps-fedora
deps-ubuntu:
	sudo apt-get update -qq
	sudo apt-get install -y \
	    build-essential cmake pkg-config \
	    libmupdf-dev libqpdf-dev \
	    zlib1g-dev libpng-dev libjpeg-turbo8-dev \
	    libfreetype6-dev libharfbuzz-dev \
	    libmujs-dev libgumbo-dev \
	    libjbig2dec0-dev libopenjp2-7-dev \
	    libbrotli-dev libbz2-dev
	@echo "  ✓  All build dependencies installed"

deps-arch:
	sudo pacman -S --needed \
	    base-devel cmake pkgconf \
	    mupdf-tools qpdf zlib libpng libjpeg-turbo \
	    freetype2 harfbuzz mujs gumbo jbig2dec openjpeg2 brotli go

deps-fedora:
	sudo dnf install -y \
	    gcc-c++ cmake pkg-config \
	    mupdf-devel qpdf-devel zlib-devel \
	    libpng-devel libjpeg-turbo-devel \
	    freetype-devel harfbuzz-devel \
	    jbig2dec-devel openjpeg2-devel brotli-devel golang

# ═══════════════════════════════════════════════════════════════════════
.PHONY: info help
info:
	@echo "Platform    : $(PLATFORM)"
	@echo "CMake gen   : $(CMAKE_GEN)"
	@echo "CPU cores   : $(NPROC)"
	@echo "MuPDF found : $(MUPDF_FOUND)"
	@echo "QPDF found  : $(QPDF_FOUND)"
	@echo "Go          : $(shell go version 2>/dev/null || echo NOT FOUND)"
	@echo "GCC         : $(shell gcc --version 2>/dev/null | head -1 || echo NOT FOUND)"
	@echo "CGO_LDFLAGS : $(CGO_LDFLAGS)"

help:
	@echo "Targets: all install clean distclean dist test info"
	@echo "         deps-ubuntu deps-arch deps-fedora"
