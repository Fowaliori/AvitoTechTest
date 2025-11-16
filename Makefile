GOPATH_BIN = $(shell go env GOPATH)/bin
OAPI_CODEGEN = $(GOPATH_BIN)/oapi-codegen
API_FILE = internal/api/api.gen.go
MODELS_FILE = internal/models/models.gen.go

all: generate linter run

generate: install-tools
	$(GOPATH_BIN)/oapi-codegen -package models -generate types -o internal/models/models.gen.go openapi.yml
	$(GOPATH_BIN)/oapi-codegen -config oapi-config.yaml -o internal/api/api.gen.go openapi.yml

install-tools:
	@CGO_ENABLED=0 GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2
	@CGO_ENABLED=0 GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) go install github.com/tsenart/vegeta/v12@v12.11.1


run:
	@docker-compose up --build -d
	
test-e2e:
	@go test -v ./e2e_test.go

linter:
	@$(GOPATH_BIN)/golangci-lint run

.PHONY: all generate install-tools run test-e2e linter 