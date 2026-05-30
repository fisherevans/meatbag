.PHONY: build ui go test clean dev sync-guide install overlay codesign-setup

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

SIGN_ID   := meatbag-codesign
SIGN_CERT := /tmp/meatbag-cs.crt
SIGN_KEY  := /tmp/meatbag-cs.key
INSTALLED := $(shell which meatbag 2>/dev/null || echo ~/.local/bin/meatbag)

# install: build a fresh binary then run its own `install` subcommand to
# atomically swap the on-PATH copy (default ~/.local/bin/meatbag). Override the
# destination by passing INSTALL_ARGS, e.g.:
#   make install INSTALL_ARGS="--target=/usr/local/bin"
# After install, re-sign with the stable cert so macOS doesn't re-prompt for
# Keychain access. Run 'make codesign-setup' once first if you haven't already.
install: build
	./$(BIN) install $(INSTALL_ARGS)
	@if security find-identity -p codesigning 2>/dev/null | grep -q "$(SIGN_ID)"; then \
		codesign --force --sign "$(SIGN_ID)" "$(INSTALLED)" && \
		echo "codesigned $(INSTALLED)"; \
	else \
		echo "tip: run 'make codesign-setup' once to stop repeated Keychain prompts"; \
	fi

# codesign-setup: create a self-signed code signing cert and import it into the
# login keychain. Run once; subsequent 'make install' calls will re-sign with it.
# You may be prompted for your login keychain password once during import.
codesign-setup:
	@if security find-identity -p codesigning 2>/dev/null | grep -q "$(SIGN_ID)"; then \
		echo "$(SIGN_ID) already exists - nothing to do"; \
	else \
		echo "Creating self-signed code signing cert '$(SIGN_ID)'..."; \
		openssl req -x509 -newkey rsa:2048 \
			-keyout $(SIGN_KEY) -out $(SIGN_CERT) \
			-days 3650 -nodes \
			-subj "/CN=$(SIGN_ID)" \
			-addext "keyUsage=critical,digitalSignature" \
			-addext "extendedKeyUsage=critical,codeSigning" 2>/dev/null; \
		security import $(SIGN_KEY) \
			-k ~/Library/Keychains/login.keychain-db \
			-T /usr/bin/codesign -A; \
		security import $(SIGN_CERT) \
			-k ~/Library/Keychains/login.keychain-db; \
		security add-trusted-cert -p codeSign $(SIGN_CERT); \
		rm -f $(SIGN_KEY) $(SIGN_CERT); \
		echo "Done. Run 'make install' to sign the binary."; \
	fi

overlay:
	$(MAKE) -C macos/MeatbagOverlay all
