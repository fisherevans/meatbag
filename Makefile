.PHONY: build ui go test clean dev sync-guide install

BIN := bin/meatbag

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
	go build -o $(BIN) ./cmd/meatbag

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
