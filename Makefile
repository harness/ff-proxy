ifndef GOPATH
        GOPATH := $(shell go env GOPATH)
endif
ifndef GOBIN
        GOBIN := $(shell go env GOPATH)/bin
endif
ifndef DOCKER_BUILD_OPTS
	DOCKER_BUILD_OPTS := --build
endif

.DEFAULT_GOAL := all

tools = $(addprefix $(GOBIN)/, golangci-lint gosec goimports gocov gocov-html)
deps = $(addprefix $(GOBIN)/, oapi-codegen)
export formatlist = $(shell go list  ./... | sed 's/\github.com\/harness\/ff-proxy\///g' | sed 's/\github.com\/harness\/ff-proxy//g' | sed 's/\gen\/admin//g' | sed 's/\gen\/client//g')

ifndef GIT_TAG
	export GIT_TAG = $(shell git describe --tags --abbrev=0)
endif

dep: $(deps) ## Install the deps required to generate code and build feature flags
	@echo "Installing dependances"
	@go mod download

tools: $(tools) ## Install tools required for the build
	@echo "Installed tools"

all: dep generate build ## Pulls down required deps, runs required code generation and builds the ff-proxy binary

# Install oapi-codegen to generate ff server code from the apis
$(GOBIN)/oapi-codegen:
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.11.0

PHONY+= generate
generate: ## Generates the client for the ff-servers client service
	oapi-codegen --config ./ff-api/config/ff-proxy/client-client.yaml ./ff-api/docs/release/client-v1.yaml > gen/client/services.gen.go
	oapi-codegen --config ./ff-api/config/ff-proxy/client-types.yaml ./ff-api/docs/release/client-v1.yaml > gen/client/types.gen.go
	oapi-codegen --config ./ff-api/config/ff-proxy/admin-client.yaml  ./ff-api/docs/release/admin-v1.yaml > gen/admin/services.gen.go
	oapi-codegen --config ./ff-api/config/ff-proxy/admin-types.yaml ./ff-api/docs/release/admin-v1.yaml > gen/admin/types.gen.go


PHONY+= build
build: ## Builds the ff-proxy service binary
	CGO_ENABLED=0 go build -ldflags="-X github.com/harness/ff-proxy/build.Version=${GIT_TAG}" -o ff-proxy ./cmd/ff-proxy/main.go

PHONY+= build-race
build-race: generate ## Builds the ff-proxy service binary with the race detector enabled
	CGO_ENABLED=1 go build -race -o ff-proxy ./cmd/ff-proxy/main.go

image: ## Builds a docker image for the proxy called ff-proxy:latest
	@echo "Building Feature Flag Proxy Image"
	@docker build -t harness/ff-proxy:latest -f ./Dockerfile .

PHONY+= test
test: ## Run the go tests (runs with race detector enabled)
	@echo "Running tests"
	go test -race -v -coverprofile=coverage.out $(go list ./... | grep -v /tests)
	go tool cover -html=coverage.out

PHONY+= integration-test
integration-test: ## Brings up pushpin & redis and runs go tests (runs with race detector enabled)
	@echo "Running tests"
	make dev
	go test -short -race -v -coverprofile=coverage.out $(go list ./... | grep -v /tests)
	go tool cover -html=coverage.out
	make stop


###########################################
# we use -coverpkg command to report coverage for any line run from any package
# the input for this param is a comma separated list of all packages in our repo excluding the /cmd/ and /gen/ directories
###########################################
test-report: ## Run the go tests and generate a coverage report
	@echo "Running tests"
	go test -covermode=atomic -coverprofile=proxy.cov -coverpkg=$(shell go list ./... | grep -v /cmd/ | grep -v /gen/ | xargs | sed -e 's/ /,/g') $(go list ./... | grep -v /tests)
	gocov convert ./proxy.cov | gocov-html > ./proxy_test_coverage.html

PHONY+= dev
dev: ## Brings up services that the proxy uses
	docker-compose -f ./docker-compose.yml up -d --remove-orphans redis pushpin

generate-e2e-env-files: ## Generates the .env files needed to run the e2e tests below
	go run tests/e2e/testhelpers/setup/main.go

e2e-offline-redis: ## brings up offline proxy in redis mode and runs e2e sdk tests against it
	OFFLINE=true AUTH_SECRET=my_secret REDIS_ADDRESS=redis:6379 CONFIG_VOLUME=./tests/e2e/testdata/config:/config docker-compose -f ./docker-compose.yml up -d --remove-orphans proxy redis
	sleep 5
	go test -p 1 -v ./tests/... -env=".env.offline" | tee /dev/stderr | go-junit-report -set-exit-code > report.xml

e2e-offline-in-mem: ## brings up offline proxy in in-memory mode and runs e2e sdk tests against it
	OFFLINE=true AUTH_SECRET=my_secret CONFIG_VOLUME=./tests/e2e/testdata/config:/config docker-compose -f ./docker-compose.yml up -d --remove-orphans proxy
	sleep 5
	go test -p 1 -v ./tests/... -env=".env.offline" | tee /dev/stderr | go-junit-report -set-exit-code > report.xml

e2e-online-in-mem: ## brings up proxy in online in memory mode and runs e2e sdk tests against it
	docker-compose --env-file .env.online_in_mem -f ./docker-compose.yml up -d --remove-orphans proxy
	sleep 5 ## TODO replace with a check for the proxy and all envs being healthy
	RUN_METRICS_TESTS=true STREAM_URL=https://localhost:7000 go test -p 1 -v ./tests/... -env=".env.online" | tee /dev/stderr | go-junit-report -set-exit-code > online-in-memory.xml

e2e-online-redis: ## brings up proxy in online in redis mode and runs e2e sdk tests against it
	docker-compose --env-file .env.online_redis -f ./docker-compose.yml up -d --remove-orphans proxy redis
	sleep 5s  ## TODO replace with a check for the proxy and all envs being healthy
	go test -p 1 -v ./tests/... -env=".env.online" | tee /dev/stderr | go-junit-report -set-exit-code > online-redis.xml

e2e-generate-offline-config: ## brings up proxy to generate offline config then runs in offline mode
	CONFIG_VOLUME=./testconfig:/config docker-compose --env-file .env.generate_offline -f ./docker-compose.yml up -d --remove-orphans proxy
	sleep 5 ## TODO replace with a check that ./testconfig has been populated with data
	CONFIG_VOLUME=./testconfig:/config docker-compose --env-file .env.offline -f ./docker-compose.yml up -d --remove-orphans proxy
	ONLINE=false go test -p 1 -v ./tests/... -env=".env.online" | tee /dev/stderr | go-junit-report -set-exit-code > generate-offline-redis.xml

PHONY+= run
run: ## Runs the proxy and redis
	docker-compose --env-file .offline.docker.env -f ./docker-compose.yml up ${DOCKER_BUILD_OPTS} --remove-orphans redis proxy

PHONY+= stop
stop: ## Stops all services brought up by make run
	docker-compose -f ./docker-compose.yml down --remove-orphans

PHONY+= clean-redis
clean-redis: ## Removes all data from redis
	redis-cli -h localhost -p 6379 flushdb

PHONY+= build-example-sdk
build-example-sdk: ## builds an example sdk that can be used for hitting the proxy
	CGO_ENABLED=0 go build -o ff-example-sdk ./cmd/example-sdk/main.go


#########################################
# lint
# These lint, format and check the code for potential vulnerabilities
#########################################
PHONY+= lint
lint: tools ## lint the golang code
	@echo "Linting $(1)"
	@golangci-lint run ./...

PHONY+= tools
format: tools ## Format go code and error if any changes are made
	@echo "Formatting ..."
	@goimports -l $$formatlist
	@goimports -l -w $$formatlist | wc -m | xargs | grep -q 0
	@echo "Formatting complete"

###########################################
# Install Tools and deps
#
# These targets specify the full path to where the tool is installed
# If the tool already exists it wont be re-installed.
###########################################

# Install golangci-lint
$(GOBIN)/golangci-lint:
	@echo "🔘 Installing golangci-lint... (`date '+%H:%M:%S'`)"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2

# Install goimports to format code
$(GOBIN)/goimports:
	@echo "🔘 Installing goimports ... (`date '+%H:%M:%S'`)"
	@go install golang.org/x/tools/cmd/goimports@latest

# Install gocov to parse code coverage
$(GOBIN)/gocov:
	@echo "🔘 Installing gocov ... (`date '+%H:%M:%S'`)"
	@go install github.com/axw/gocov/gocov@latest

# Install gocov-html to generate a code coverage html file
$(GOBIN)/gocov-html:
	@echo "🔘 Installing gocov-html ... (`date '+%H:%M:%S'`)"
	@go install github.com/matm/gocov-html/cmd/gocov-html@latest

# Install gosec for security scans
$(GOBIN)/gosec:
	@echo "🔘 Installing gosec ... (`date '+%H:%M:%S'`)"
	@curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(GOPATH)/bin

help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% 0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: $(PHONY)
