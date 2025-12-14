SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

RELEASE_MATRIX ?= darwin/amd64 darwin/arm64 linux/386 linux/amd64 linux/arm linux/arm64 windows/386 windows/amd64

GOTAGS      ?= forceposix
GOFLAGS     ?= -trimpath
LDFLAGS     ?= -s -w
GOWORK      ?= off
CGO_ENABLED ?= 0

BIN_NAME    ?= zenit
WORK_DIR    ?= ./cmd/zenit
BIN_DIR     ?= build

NATIVE_GOOS      := $(shell go env GOOS)
NATIVE_GOARCH    := $(shell go env GOARCH)
NATIVE_EXTENSION := $(if $(filter $(NATIVE_GOOS),windows),.exe,)

GO            ?= go
GOLANGCI_LINT ?= golangci-lint
BETTERALIGN   ?= betteralign
CYCLONEDX     ?= cyclonedx-gomod

# Container settings
GH_USER         ?= woozymasta
DOCKER_USER     ?= woozymasta
REGISTRY_GHCR   := ghcr.io/$(GH_USER)
REGISTRY_DOCKER := docker.io/$(DOCKER_USER)
IMAGE_NAME      := zenit
IMG_BASE_GHCR   := $(REGISTRY_GHCR)/$(IMAGE_NAME)
IMG_BASE_DOCKER := $(REGISTRY_DOCKER)/$(IMAGE_NAME)

# Versioning details
MODULE    := $(shell $(GO) list -m)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REVISION  := $(shell git rev-list --count HEAD 2>/dev/null || echo 0)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
NAME      := $(shell echo $(BIN_NAME) | sed 's/[-_]/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($$i,1,1)),$$i)}1')

# Linker flags to inject variables into internal/vars
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/vars.Name=$(NAME)' \
	-X '$(MODULE)/internal/vars.Version=$(VERSION)' \
	-X '$(MODULE)/internal/vars._revision=$(REVISION)' \
	-X '$(MODULE)/internal/vars.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/vars._buildTime=$(BUILDTIME)' \
	-X '$(MODULE)/internal/vars.URL=https://$(MODULE)'

HAVE_LINT  := $(shell command -v $(GOLANGCI_LINT) >/dev/null 2>&1 && echo yes || echo no)
HAVE_ALIGN := $(shell command -v $(BETTERALIGN)   >/dev/null 2>&1 && echo yes || echo no)

.DEFAULT_GOAL := build

.PHONY: all build container container-push release changelog deps clean build-dir fmt vet lint align align-fix lint check tools geodb

all: tools check build

build: clean build-dir minify
	@goos="$(GOOS)"; goarch="$(GOARCH)"; \
	GOOS=$(NATIVE_GOOS) GOARCH=$(NATIVE_GOARCH) \
	GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
		$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" -tags $(GOTAGS) $(EXTRA_BUILD_FLAGS) \
			-o $(BIN_DIR)/$(BIN_NAME)$(NATIVE_EXTENSION) $(WORK_DIR)

container:
	@echo ">> Building container image ($(IMG_BASE_GHCR):$(VERSION))..."
	@docker build -f Dockerfile \
		-t $(IMG_BASE_GHCR):$(VERSION) \
		$(if $(filter-out dev,$(VERSION)),-t $(IMG_BASE_GHCR):latest,) \
		-t $(IMG_BASE_DOCKER):$(VERSION) \
		$(if $(filter-out dev,$(VERSION)),-t $(IMG_BASE_DOCKER):latest,) \
		.

container-push:
	@echo ">> Pushing container image..."
	docker push $(IMG_BASE_GHCR):$(VERSION)
	docker push $(IMG_BASE_DOCKER):$(VERSION)
ifneq ($(VERSION),dev)
	docker push $(IMG_BASE_GHCR):latest
	docker push $(IMG_BASE_DOCKER):latest
endif

release: clean build-dir minify
	@echo ">> Building release binaries..."
	@for target in $(RELEASE_MATRIX); do \
		goos=$${target%%/*}; \
		goarch=$${target##*/}; \
		ext=$$( [ $$goos = "windows" ] && echo ".exe" || echo "" ); \
		out="$(BIN_DIR)/$(BIN_NAME)-$${goos}-$${goarch}$$ext"; \
		echo ">> building $$out"; \
		GOOS=$$goos GOARCH=$$goarch \
		GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
			$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" -tags $(GOTAGS) -o $$out $(WORK_DIR) ; \
		$(MAKE) _sbom_bin_one GOOS=$$goos GOARCH=$$goarch BIN=$(BIN_NAME)-$${goos}-$${goarch} OUTEXT="$$ext"; \
	done
	@$(MAKE) sbom-app
	@echo ">> Release build complete."

changelog:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } found { print } \
	' CHANGELOG.md

deps:
	@echo ">> Downloading dependencies..."
	@go mod tidy
	@go mod download

clean:
	@echo ">> Cleaning..."
	@rm -rf $(BIN_DIR)

build-dir:
	@echo ">> Make $(BIN_DIR) dir..."
	@mkdir -p $(BIN_DIR)

fmt:
	@echo ">> Running go fmt..."
	@go fmt ./...

vet:
	@echo ">> Running go vet..."
	@go vet ./...

align:
	@echo ">> Checking struct alignment..."
	@betteralign ./...

align-fix:
	@echo ">> Optimizing struct alignment..."
	@betteralign -apply ./...

lint:
	@echo ">> Running golangci-lint..."
	@golangci-lint run

check: fmt vet align lint
	@echo ">> All checks passed."

minify:
	@echo ">> Minify static assets."
	@for f in assets/*.html assets/*/*.{css,js,html,json}; do \
		if [[ -f "$$f" && "$${f#*.}" != min* ]]; then \
			minify "$$f" > "$${f%.*}.min.$${f#*.}"; \
		fi; \
	done

sbom-app:
	@echo ">> SBOM (app)"
	$(CYCLONEDX) app -json -packages -files -licenses \
		-output "$(BIN_DIR)/$(BIN_NAME).sbom.json" -main $(WORK_DIR)

tools: tool-golangci-lint tool-betteralign tool-cyclonedx tool-minify
	@echo ">> installing all go tools"

tool-golangci-lint:
	@echo ">> installing golangci-lint"
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	@echo ">> installing betteralign"
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-cyclonedx:
	@echo ">> installing cyclonedx-gomod"
	$(GO) install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

tool-minify:
	@echo ">> installing minify"
	$(GO) install github.com/tdewolff/minify/v2/cmd/minify@latest

geodb:
	@echo ">>> downloading GeoLite2-City.mmdb"
	curl -#SfLo GeoLite2-City.mmdb https://git.io/GeoLite2-City.mmdb
	@echo ">>> downloading GeoLite2-Country.mmdb"
	curl -#SfLo GeoLite2-Country.mmdb https://git.io/GeoLite2-Country.mmdb

# helpers
_sbom_bin_one:
	@bin="$(BIN_DIR)/$(BIN)$(OUTEXT)"; \
	if [ -f "$$bin" ]; then \
		echo ">> SBOM (bin) $$bin"; \
		$(CYCLONEDX) bin -json -output "$$bin.sbom.json" "$$bin"; \
	fi
