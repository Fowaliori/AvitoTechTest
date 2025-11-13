GOPATH_BIN = $(shell go env GOPATH)/bin
OAPI_CODEGEN = $(GOPATH_BIN)/oapi-codegen
API_FILE = internal/api/api.gen.go
MODELS_FILE = internal/models/models.gen.go

all: generate

generate: install-generator
	$(OAPI_CODEGEN) -package models -generate types -o internal/models/models.gen.go openapi.yml
	$(OAPI_CODEGEN) -config oapi-config.yaml -o internal/api/api.gen.go openapi.yml

install-generator:
	@CGO_ENABLED=0 GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

init-db:
	psql -U postgres -h localhost -p 5433 -c "CREATE DATABASE avitotech;" postgres
	goose -dir internal/db/migrate postgres "postgres://postgres:123@localhost:5433/avitotech?sslmode=disable" up

.PHONY: all generate install-generator