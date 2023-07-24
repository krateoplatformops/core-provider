# Set the shell to bash always
SHELL := /bin/bash

# Look for a .env file, and if present, set make variables from it.
ifneq (,$(wildcard ./.env))
	include .env
	export $(shell sed 's/=.*//' .env)
endif

KIND_CLUSTER_NAME ?= local-dev
KUBECONFIG ?= $(HOME)/.kube/config

VERSION := $(shell git describe --always --tags | sed 's/-/./2' | sed 's/-/./2')
ifndef VERSION
VERSION := 0.0.0
endif

# Tools
KIND=$(shell which kind)
LINT=$(shell which golangci-lint)
KUBECTL=$(shell which kubectl)
HELM=$(shell which helm)


.DEFAULT_GOAL := help


.PHONY: test
test: ## Run all the Go test
	go test -v ./...

.PHONY: lint
lint: ## Check the Go coding conventions.
	$(LINT) run

.PHONY: tidy
tidy: ## Ensure that all Go imports are satisfied.
	go mod tidy

.PHONY: generate
generate: tidy ## Generate all CRDs.
	go generate ./...

.PHONY: dev
dev: generate ## Run the controller in debug mode.
	$(KUBECTL) create namespace krateo-system || true
	$(KUBECTL) apply -f crds/ -R
	go run cmd/main.go -d

.PHONY: kind-up
kind-up: ## Starts a KinD cluster for local development.
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || $(KIND) create cluster --name=$(KIND_CLUSTER_NAME)

.PHONY: kind-down
kind-down: ## Shuts down the KinD cluster.
	@$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME)


.PHONY: help
help: ## Print this help.
	@grep -E '^[a-zA-Z\._-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'