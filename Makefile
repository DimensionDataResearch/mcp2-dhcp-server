VERSION = 0.1.0-alpha5

SERVER_DIRECTORY	= ./server
BIN_DIRECTORY   = ./_bin
EXECUTABLE_NAME = mcp2-dhcp-server
DIST_ZIP_PREFIX = $(EXECUTABLE_NAME).v$(VERSION)
VERSION_INFO_FILE = $(SERVER_DIRECTORY)/version-info.go

REPO_BASE	= github.com/DimensionDataResearch
REPO_ROOT	= $(REPO_BASE)/mcp2-dhcp-server
SERVER_ROOT	= $(REPO_ROOT)/server

default: fmt build test

fmt:
	go fmt $(SERVER_ROOT)/...

clean:
	rm -rf $(BIN_DIRECTORY) $(VERSION_INFO_FILE)
	go clean $(SERVER_ROOT)/...

# Peform a development (current-platform-only) build.
dev: version fmt
	go build -o $(BIN_DIRECTORY)/$(EXECUTABLE_NAME) $(SERVER_ROOT)

# Perform a full (all-platforms) build.
build: version build-linux64 build-mac64

build-linux64: version
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIRECTORY)/linux-amd64/$(EXECUTABLE_NAME) $(SERVER_ROOT)

build-mac64: version
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIRECTORY)/darwin-amd64/$(EXECUTABLE_NAME) $(SERVER_ROOT)

# Produce archives for a GitHub release.
dist: build
	cd $(BIN_DIRECTORY)/linux-amd64 && \
		zip -9 ../$(DIST_ZIP_PREFIX).linux-amd64.zip $(EXECUTABLE_NAME)
	cd $(BIN_DIRECTORY)/darwin-amd64 && \
		zip -9 ../$(DIST_ZIP_PREFIX)-darwin-amd64.zip $(EXECUTABLE_NAME)

test:
	go test $(SERVER_ROOT)

# Version info.
version: $(VERSION_INFO_FILE)

$(VERSION_INFO_FILE): Makefile
	@echo "Update version info: v$(VERSION)"
	@echo "package main\n\n// ProductVersion is the current version of the MCP 2.0 DHCP server.\nconst ProductVersion = \"v$(VERSION) (`git rev-parse HEAD`)\"" > $(VERSION_INFO_FILE)
