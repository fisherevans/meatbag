APP        := MeatbagOverlay
BUILD_DIR  := build
APP_BUNDLE := $(BUILD_DIR)/$(APP).app
CONTENTS   := $(APP_BUNDLE)/Contents
BINARY     := $(CONTENTS)/MacOS/$(APP)
SOURCES    := $(wildcard Sources/*.swift)

# Build for the host architecture.
ARCH := $(shell uname -m)
ifeq ($(ARCH),arm64)
    TARGET := arm64-apple-macosx12.0
else
    TARGET := x86_64-apple-macosx12.0
endif

SWIFTC_FLAGS := \
    -target $(TARGET) \
    -framework Cocoa \
    -framework WebKit

.PHONY: all clean run register

all: $(APP_BUNDLE)

$(APP_BUNDLE): $(SOURCES) Info.plist
	mkdir -p $(CONTENTS)/MacOS $(CONTENTS)/Resources
	swiftc $(SWIFTC_FLAGS) $(SOURCES) -o $(BINARY)
	cp Info.plist $(CONTENTS)/Info.plist
	@echo "Built: $(APP_BUNDLE)"
	@echo "Run 'make register' to register the meatbag:// URL scheme, or just run 'make run' once."

# Open the app to trigger Launch Services registration of the URL scheme.
register run: $(APP_BUNDLE)
	open $(APP_BUNDLE)

clean:
	rm -rf $(BUILD_DIR)
