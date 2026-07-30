.PHONY: build dev frontend-build frontend-dev install restart

build: frontend-build
	go build -o claude-tabs .

dev:
	go run . --server

frontend-dev:
	cd frontend && pnpm dev

frontend-build:
	cd frontend && pnpm install && pnpm build

install: build
	mkdir -p ~/.claude-tabs/bin
	cp claude-tabs ~/.claude-tabs/bin/claude-tabs

restart: install
	-pkill -f "claude-tabs --server"
	~/.claude-tabs/bin/claude-tabs

clean:
	rm -f claude-tabs
	rm -rf frontend/dist
