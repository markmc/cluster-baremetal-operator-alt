DBG         ?= 0
USERNAME    ?= $$USER
REGISTRY    ?= quay.io/$(USERNAME)/
VERSION     ?= v0.0.1
IMAGE        = $(REGISTRY)cluster-baremetal-operator

# Enable go modules and vendoring
# https://github.com/golang/go/wiki/Modules#how-to-install-and-activate-module-support
# https://github.com/golang/go/wiki/Modules#how-do-i-use-vendoring-with-modules-is-vendoring-going-away
GO111MODULE = on
export GO111MODULE
GOFLAGS ?= -mod=vendor
export GOFLAGS

ifeq ($(DBG),1)
GOGCFLAGS ?= -gcflags=all="-N -l"
endif

.PHONY: all
all: build check unit

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: check
check: lint fmt vet verify-codegen ## Run code validations

.PHONY: build
build: cluster-baremetal-operator ## Build binaries

.PHONY: cluster-baremetal-operator
cluster-baremetal-operator:
	./hack/go-build.sh

.PHONY: generate
generate: gen-crd update-codegen

.PHONY: gen-crd
gen-crd:
	operator-sdk generate crds

.PHONY: update-codegen
update-codegen:
	operator-sdk generate k8s

.PHONY: verify-codegen
verify-codegen:
	./hack/verify-codegen.sh

unit:
	go test ./pkg/... ./cmd/...

.PHONY: image
image: ## Build docker image
	@echo -e "\033[32mBuilding image $(IMAGE):$(VERSION)...\033[0m"
	operator-sdk build "$(IMAGE):$(VERSION)"

.PHONY: push
push: ## Push image to docker registry
	@echo -e "\033[32mPushing images...\033[0m"
	docker push "$(IMAGE):$(VERSION)"

.PHONY: lint
lint: ## Go lint your code
	hack/go-lint.sh -min_confidence 0.3 $(go list -f '{{ .ImportPath }}' ./...)

.PHONY: fmt
fmt: ## Go fmt your code
	hack/go-fmt.sh .

.PHONY: goimports
goimports: ## Go fmt your code
	hack/goimports.sh .

.PHONY: vet
vet: ## Apply go vet to all go files
	hack/go-vet.sh ./...

.PHONY: help
help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
