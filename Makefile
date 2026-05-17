.PHONY: build ui go test clean dev sync-guide install

BIN := bin/meatbag

# Version metadata injected into the binary via -ldflags. `git describe`
# yields the most recent tag plus a -N-g<sha> suffix on commits past the tag,
# or -dirty if the worktree is unclean. The fallbacks let `go run`/`go build`
# without make still produce a usable binary.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/fisherevans/meatbag/internal/version.Version=$(VERSION) \
           -X github.com/fisherevans/meatbag/internal/version.Commit=$(COMMIT) \
           -X github.com/fisherevans/meatbag/internal/version.Date=$(DATE)

build: sync-guide ui go

# Keep the embedded copy of the agent guide in sync with the canonical
# docs/agent-guide.md, then build everything.
sync-guide:
	cp docs/agent-guide.md internal/cli/agent_help.md

ui:
	cd ui && npm install --silent && npm run build
	rm -rf internal/ui/dist
	mkdir -p internal/ui/dist
	cp -R ui/dist/. internal/ui/dist/

go:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/meatbag

test:
	go test ./...

dev:
	cd ui && npm run dev

clean:
	rm -rf bin internal/ui/dist ui/dist ui/node_modules

# install: build a fresh binary then run its own `install` subcommand to
# atomically swap the on-PATH copy (default ~/.local/bin/meatbag). Override the
# destination by passing INSTALL_ARGS, e.g.:
#   make install INSTALL_ARGS="--target=/usr/local/bin"
install: build
	./$(BIN) install $(INSTALL_ARGS)
