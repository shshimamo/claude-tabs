.PHONY: build dev frontend-build frontend-dev install restart setup-hooks clean

# make build              → darwin/arm64 (auto detect)
# GOOS=linux GOARCH=amd64 make build → linux/amd64
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

build: frontend-build
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o claude-tabs .

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

setup-hooks:
	@if [ ! -f ~/.claude/settings.json ]; then echo '{}' > ~/.claude/settings.json; fi
	jq --slurpfile h hooks-setup.json ' \
		.hooks //= {} | \
		reduce ($$h[0].hooks | to_entries[]) as $$e (.; \
			if (.hooks[$$e.key] // [] | any(.hooks[].command | contains("claude-tabs"))) \
			then . \
			else .hooks[$$e.key] = ((.hooks[$$e.key] // []) + $$e.value) \
			end \
		)' ~/.claude/settings.json > ~/.claude/settings.json.tmp && mv ~/.claude/settings.json.tmp ~/.claude/settings.json
	@echo "hooks added to ~/.claude/settings.json"

clean:
	rm -f claude-tabs
	rm -rf frontend/dist
