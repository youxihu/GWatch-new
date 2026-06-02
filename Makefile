.PHONY: all run wire build build_arm docker docker_arm docker_push clean help

APP_NAME    := Gwatch
BIN_DIR     := bin
CMD_DIR     := cmd
MAIN_FILE   := $(CMD_DIR)/main.go
WIRE_FILE   := $(CMD_DIR)/wire_gen.go
IMAGE_NAME  := youxihu/gwatch

VERSION     := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo dev)
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_AUTHOR  := $(shell git log -1 --pretty=format:'%an' 2>/dev/null || echo unknown)
BUILD_TIME  := $(shell date +"%Y-%m-%d %H:%M:%S")

GOOS        ?= linux
GOARCH      ?= amd64
CGO_ENABLED ?= 0

LDFLAGS := -s -w \
	-X 'GWatch/internal/utils.Version=$(VERSION)' \
	-X 'GWatch/internal/utils.GitCommit=$(GIT_COMMIT)' \
	-X 'GWatch/internal/utils.GitAuthor=$(GIT_AUTHOR)' \
	-X 'GWatch/internal/utils.BuildTime=$(BUILD_TIME)'

all: help

run:
	go run ./$(CMD_DIR)

wire:
	wire ./$(CMD_DIR)

build: 
	@echo "▶ 开始编译 amd64 二进制"
	rm -rf $(BIN_DIR)
	mkdir -p $(BIN_DIR)/linux_amd64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/linux_amd64/$(APP_NAME) \
		$(MAIN_FILE) $(WIRE_FILE)
	@if command -v upx >/dev/null 2>&1; then \
		upx -9 $(BIN_DIR)/linux_amd64/$(APP_NAME) && echo "▶ upx 压缩完成"; \
	else \
		echo "⚠ 未安装 upx，跳过压缩"; \
	fi
	@du -sh $(BIN_DIR)/linux_amd64/$(APP_NAME)
	@echo "✅ amd64 编译完成:"

build_arm: 
	@echo "▶ 开始编译 arm64 二进制"
	rm -rf $(BIN_DIR)/linux_arm64
	mkdir -p $(BIN_DIR)/linux_arm64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 \
	go build -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/linux_arm64/$(APP_NAME) \
		$(MAIN_FILE) $(WIRE_FILE)
	@if command -v upx >/dev/null 2>&1; then \
		upx -9 $(BIN_DIR)/linux_arm64/$(APP_NAME) && echo "▶ arm64 upx 压缩完成"; \
	else \
		echo "⚠ 未安装 upx，跳过 arm64 压缩"; \
	fi
	@du -sh $(BIN_DIR)/linux_arm64/$(APP_NAME)
	@echo "✅ arm64 编译完成:"

docker:
	docker build \
		-f docker/Dockerfile \
		--build-arg BIN_FILE=$(BIN_DIR)/$(APP_NAME) \
		-t $(IMAGE_NAME):$(VERSION)-amd64 \
		.

docker_push:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-f docker/Dockerfile \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

clean:
	rm -rf $(BIN_DIR)

help:
	@echo ""
	@echo "可用命令："
	@echo "  make run       本地运行"
	@echo "  make wire      生成 wire 代码"
	@echo "  make build     编译 amd64 二进制"
	@echo "  make build_arm 编译 arm64 二进制"
	@echo "  make docker    构建 amd64 镜像"
	@echo "  make docker_arm 构建 arm64 镜像"
	@echo "  make docker_push 推送 amd64 和 arm64 镜像"
	@echo "  make clean     清理产物"
	@echo ""
