.PHONY: build ui go test clean dev sync-guide

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
