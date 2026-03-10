# BatchSync AI - Makefile

# Cargar variables de entorno desde .env si existe.
ifneq (,$(wildcard .env))
	include .env
	export
endif

.DEFAULT_GOAL := help

.PHONY: help build run test test-unit test-integration test-pkg test-race fmt vet tidy deps clean status pull push sync

# Variables
APP_NAME   := batchsync
BUILD_DIR  := build
CMD_DIR    := ./cmd/$(APP_NAME)
GO_CACHE   := /tmp/batchsync-gocache
BRANCH     := $(shell git branch --show-current)

# Ayuda
help: ## Muestra esta ayuda
	@echo ""
	@echo "  BatchSync AI - Comandos disponibles"
	@echo ""
	@echo "  BUILD & RUN"
	@grep -h -E '^(build|run).*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  TEST"
	@grep -h -E '^test.*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  QUALITY"
	@grep -h -E '^(fmt|vet|tidy|deps).*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  GIT ($(BRANCH))"
	@grep -h -E '^(status|pull|push|sync).*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  CLEANUP"
	@grep -h -E '^clean.*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  Ejemplos:"
	@echo "    make build"
	@echo "    make run"
	@echo "    make test"
	@echo "    make test-pkg PKG=./internal/processor"
	@echo "    make push m='mensaje de commit'"
	@echo ""

# Build & Run
build: ## Compila el binario en ./build/batchsync
	@echo "Compilando $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@GOCACHE=$(GO_CACHE) go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Binario generado en $(BUILD_DIR)/$(APP_NAME)"

run: ## Ejecuta el job batch una sola vez
	@echo "Ejecutando $(APP_NAME)..."
	@GOCACHE=$(GO_CACHE) go run $(CMD_DIR)

# Tests
test: ## Ejecuta todos los tests con race detector
	@echo "Ejecutando tests..."
	@GOCACHE=$(GO_CACHE) GEMINI_API_KEY= SQLSERVER_CONN_STRING= go test -race ./...

test-unit: ## Ejecuta tests unitarios sin depender de servicios externos
	@echo "Ejecutando tests unitarios..."
	@GOCACHE=$(GO_CACHE) GEMINI_API_KEY= SQLSERVER_CONN_STRING= go test -race -run 'Test[^_]+$$' ./...

test-integration: ## Ejecuta tests con salida verbose; Gemini se omite si falta GEMINI_API_KEY
	@if [ -z "$(GEMINI_API_KEY)" ]; then \
		echo "\033[33mWARN: GEMINI_API_KEY no está configurada; los tests de integración con Gemini se omitirán\033[0m"; \
	fi
	@if [ -z "$(SQLSERVER_CONN_STRING)" ]; then \
		echo "\033[33mWARN: SQLSERVER_CONN_STRING no está configurada; no hay tests de integración de BD hoy, pero el runtime fallará sin ella\033[0m"; \
	fi
	@echo "Ejecutando tests de integración..."
	@GOCACHE=$(GO_CACHE) go test -race -v ./...

test-pkg: ## Ejecuta tests de un paquete específico; usar PKG=./internal/processor
	@if [ -z "$(PKG)" ]; then \
		echo "\033[31mERROR: Debes indicar PKG. Ejemplo: make test-pkg PKG=./internal/processor\033[0m"; \
		exit 1; \
	fi
	@echo "Ejecutando tests de $(PKG)..."
	@GOCACHE=$(GO_CACHE) GEMINI_API_KEY= SQLSERVER_CONN_STRING= go test -race -v $(PKG)

test-race: ## Ejecuta tests orientados a concurrencia en processor
	@echo "Ejecutando tests de concurrencia..."
	@GOCACHE=$(GO_CACHE) GEMINI_API_KEY= SQLSERVER_CONN_STRING= go test -race -v -run 'TestProcesar|TestRace|TestWorker|TestGoroutine' ./internal/processor/...

# Calidad
fmt: ## Formatea el código Go
	@echo "Formateando código..."
	@gofmt -w $$(find cmd internal -name '*.go' -type f)

vet: ## Ejecuta go vet sobre todos los paquetes
	@echo "Analizando código..."
	@GOCACHE=$(GO_CACHE) go vet ./...

tidy: ## Limpia y sincroniza go.mod y go.sum
	@echo "Ejecutando go mod tidy..."
	@GOCACHE=$(GO_CACHE) go mod tidy

deps: ## Descarga dependencias del módulo
	@echo "Descargando dependencias..."
	@GOCACHE=$(GO_CACHE) go mod download

# Git
status: ## Muestra el estado de git
	@echo "Estado de git (rama: $(BRANCH))"
	@git status

pull: ## Hace pull de la rama actual
	@echo "Actualizando desde origin/$(BRANCH)..."
	@git fetch origin
	@git pull --ff-only origin $(BRANCH)

push: ## Commit + push; requiere m='mensaje'
	@if [ -z "$(m)" ]; then \
		echo "\033[31mERROR: Debes indicar m='mensaje de commit'\033[0m"; \
		exit 1; \
	fi
	@echo "Agregando archivos..."
	@git add .
	@echo "Creando commit..."
	@git commit -m "$(m)"
	@echo "Pusheando a origin/$(BRANCH)..."
	@git push origin $(BRANCH)

sync: ## Pull --ff-only + commit + push; requiere m='mensaje'
	@if [ -z "$(m)" ]; then \
		echo "\033[31mERROR: Debes indicar m='mensaje de commit'\033[0m"; \
		exit 1; \
	fi
	@echo "Sincronizando rama $(BRANCH)..."
	@git fetch origin
	@git pull --ff-only origin $(BRANCH)
	@git add .
	@git commit -m "$(m)"
	@git push origin $(BRANCH)

# Limpieza
clean: ## Elimina binarios generados y limpia cache de tests
	@echo "Limpiando archivos generados..."
	@rm -rf $(BUILD_DIR)
	@GOCACHE=$(GO_CACHE) go clean -testcache
