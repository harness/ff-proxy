ifndef GOPATH
        GOPATH := $(shell go env GOPATH)
endif
ifndef GOBIN
        GOBIN := $(shell go env GOPATH)/bin
endif

.DEFAULT_GOAL := all

deps = $(addprefix $(GOBIN)/, oapi-codegen)

dep: $(deps) ## Install the deps required to generate code and build feature flags
	@echo "Installing dependances"
	@go mod download

tools: $(tools) ## Install tools required for the build
	@echo "Installed tools"

all: dep generate build ## Pulls down required deps, runs required code generation and builds the ff-proxy binary

# Install oapi-codegen to generate ff server code from the apis
$(GOBIN)/oapi-codegen:
	go get github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.6.0

.PHONY: generate
generate: ## Generates the client for the ff-servers client service
	oapi-codegen -generate client -package=client ./ff-api/docs/release/client-v1.yaml > gen/client/services.gen.go
	oapi-codegen -generate types -package=client ./ff-api/docs/release/client-v1.yaml > gen/client/types.gen.go
	oapi-codegen -generate client -package=admin  ./ff-api/docs/release/admin-v1.yaml > gen/admin/services.gen.go
	oapi-codegen -generate types -package=admin ./ff-api/docs/release/admin-v1.yaml > gen/admin/types.gen.go

.PHONY: build
build: generate ## Builds the ff-proxy service binary
	CGO_ENABLED=0 go build -o ff-proxy ./cmd/ff-proxy/main.go

image:
	@echo "Building Feature Flag Proxy Image"
	@docker build --build-arg GITHUB_ACCESS_TOKEN=${GITHUB_ACCESS_TOKEN} -t ff-proxy:latest -f ./Dockerfile .

.PHONY: test
test: ## Run the go tests
	@echo "Running tests"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

#########################################
# Checks
# These lint, format and check the code for potential vulnerabilities
#########################################
.PHONY: check
check: lint format sec ## Runs linter, goimports and gosec

.PHONY: lint
lint: tools ## lint the golang code
	@echo "Linting $(1)"
	@golint ./...
	@go vet ./...

.PHONY: tools
format: tools ## Format go code and error if any changes are made
	@echo "Formating ..."
	@goimports -w .
	@echo "Formatting complete"

.PHONY: sec
sec: tools ## Run the security checks
	@echo "Checking for security problems ..."
	@gosec -quiet -confidence high -severity medium ./...
	@echo "No problems found"

help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% 0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
