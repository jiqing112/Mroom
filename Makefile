# Mrome — 自托管音乐库

.PHONY: dev server web build run test docker clean

dev:            ## 同时启动 Go 服务与 Vite 开发服务器
	@echo "Go:  http://localhost:4533"
	@echo "Web: http://localhost:5173"
	@(go run ./cmd/mrome &) ; cd web && npm run dev

server:
	go run ./cmd/mrome

web:
	cd web && npm run dev

build:          ## 构建前端 + 服务端二进制
	cd web && npm install --no-audit --no-fund && npm run build
	go build -trimpath -ldflags="-s -w" -o mrome.exe ./cmd/mrome

run: build
	./mrome.exe

test:           ## 运行 Go 单元测试
	go test ./...

docker:         ## 一条命令启动
	docker compose up -d --build

clean:
	rm -rf web/dist mrome.exe
