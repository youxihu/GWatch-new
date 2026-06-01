.PHONY: run wire build deploy
#run
run:
	go run ./cmd
wire:
	wire ./cmd
# build
build: wire
	rm -rf ./bin
	mkdir -p bin/
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '\n' || echo "dev"); \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	GIT_AUTHOR=$$(git log -1 --pretty=format:'%an' 2>/dev/null || echo "unknown"); \
	BUILD_TIME=$$(date +"%Y-%m-%d %H:%M:%S"); \
	echo "构建版本: $$VERSION, 提交: $$GIT_COMMIT, 作者: $$GIT_AUTHOR, 时间: $$BUILD_TIME"; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -X 'GWatch/internal/utils.Version=$$VERSION' -X 'GWatch/internal/utils.GitCommit=$$GIT_COMMIT' -X 'GWatch/internal/utils.GitAuthor=$$GIT_AUTHOR' -X 'GWatch/internal/utils.BuildTime=$$BUILD_TIME'" \
		-o ./bin/Gwatch cmd/main.go cmd/wire_gen.go
	upx -9 ./bin/Gwatch
	echo "upx 压缩完成"
	@echo "编译完成: bin/Gwatch"
	@ls -lh bin/Gwatch
