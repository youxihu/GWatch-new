.PHONY: all run wire build docker clean help

# ======================
# 基础变量
# ======================
APP_NAME    := Gwatch
BIN_DIR     := bin
CMD_DIR     := cmd
MAIN_FILE   := $(CMD_DIR)/main.go
WIRE_FILE   := $(CMD_DIR)/wire_gen.go

VERSION     := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo dev)
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_AUTHOR  := $(shell git log -1 --pretty=format:'%an' 2>/dev/null || echo unknown)
BUILD_TIME  := $(shell date +"%Y-%m-%d %H:%M:%S")

GOOS        ?= linux
GOARCH      ?= amd64
CGO_ENABLED ?= 0

# ======================
# Go 构建参数
# ======================
LDFLAGS := -s -w \
	-X 'GWatch/internal/utils.Version=$(VERSION)' \
	-X 'GWatch/internal/utils.GitCommit=$(GIT_COMMIT)' \
	-X 'GWatch/internal/utils.GitAuthor=$(GIT_AUTHOR)' \
	-X 'GWatch/internal/utils.BuildTime=$(BUILD_TIME)'

# ======================
# 默认目标
# ======================
all: help

# ======================
# 运行
# ======================
run:
	go run ./$(CMD_DIR)

# ======================
# wire
# ======================
wire:
	wire ./$(CMD_DIR)

# ======================
# 构建
# ======================
build: wire
	@echo "▶ 构建版本: $(VERSION)"
	@echo "▶ 提交: $(GIT_COMMIT)"
	@echo "▶ 作者: $(GIT_AUTHOR)"
	@echo "▶ 时间: $(BUILD_TIME)"

	rm -rf $(BIN_DIR)
	mkdir -p $(BIN_DIR)

	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/$(APP_NAME) \
		$(MAIN_FILE) $(WIRE_FILE)

	@if command -v upx >/dev/null 2>&1; then \
		upx -9 $(BIN_DIR)/$(APP_NAME) && echo "▶ upx 压缩完成"; \
	else \
		echo "⚠ 未安装 upx，跳过压缩"; \
	fi

	@echo "✅ 编译完成: $(BIN_DIR)/$(APP_NAME)"
	@ls -lh $(BIN_DIR)/$(APP_NAME)

# ======================
# Docker
# ======================
docker:
	docker build \
		-f docker/Dockerfile \
		-t gwatch:$(VERSION) \
		.

# ======================
# 清理
# ======================
clean:
	rm -rf $(BIN_DIR)

# ======================
# 帮助
# ======================
help:
	@echo ""
	@echo "可用命令："
	@echo "  make run      本地运行"
	@echo "  make wire     生成 wire 代码"
	@echo "  make build    编译二进制"
	@echo "  make docker   构建 Docker 镜像"
	@echo "  make clean    清理产物"
	@echo ""