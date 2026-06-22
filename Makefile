# Kestrel — top-level Makefile.
# Delegates to per-component Makefiles where present, but exposes a single
# entrypoint for dev workflows (build/test/dev-up/dev-down).

SHELL          := /bin/bash
.DEFAULT_GOAL  := help

REGISTRY       ?= ghcr.io/kestrel
TAG            ?= dev
KIND_CLUSTER   ?= kestrel-dev
CHART_DIR      ?= charts/kestrel
CHART_RELEASE  ?= kestrel
NAMESPACE      ?= kestrel-system

# -------- target cluster selection --------
# CLUSTER selects where the dev/deploy targets act:
#   kind   (default) — a local kind cluster these targets create + load into
#   remote          — an existing cluster reached via REMOTE_KUBECONFIG
#                     (e.g. the kubelab k3s test cluster)
# Remote can't use `kind load`, so images must already be in a registry the
# remote nodes can pull (set REGISTRY + run `make dev-push`) and the modules
# pushed to a registry the in-cluster operator can reach (MODULE_REGISTRY +
# MODULE_SOURCE_URL). The kind defaults below keep `make dev-up` unchanged.
CLUSTER           ?= kind
REMOTE_KUBECONFIG ?= $(HOME)/kubelab.yaml
REMOTE_CONTEXT    ?= default
MODULE_SOURCE_URL      ?= kind-registry:5000
MODULE_SOURCE_INSECURE ?= true
ifeq ($(CLUSTER),remote)
KUBECONFIG_ENV := KUBECONFIG=$(REMOTE_KUBECONFIG)
else
KUBECONFIG_ENV :=
endif

GO_MODULES     := netguard operator api agent
GO_INTEGRATION_MODULES := operator api
GO_BUILDFLAGS  ?= -trimpath

# Pinned versions of coverage tooling — pulled via `go run` so no install step.
GO_TEST_COVERAGE_PKG := github.com/vladopajic/go-test-coverage/v2@v2.11.0
GOCOVMERGE_PKG       := github.com/wadey/gocovmerge@latest

# -------- help --------
.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n\nTargets:\n"} \
		/^[a-zA-Z0-9_.-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# -------- build --------
.PHONY: build
build: build-go build-web ## Build all components

.PHONY: build-go
build-go: ## Build all Go binaries
	@for m in $(GO_MODULES); do \
		echo ">> building $$m"; \
		(cd $$m && go build $(GO_BUILDFLAGS) ./...) || exit $$?; \
	done

.PHONY: build-web
build-web: ## Build web dashboard
	cd web && npm ci && npm run build

# -------- test --------
.PHONY: test
test: test-go test-web ## Run all tests

.PHONY: test-go
test-go: ## Run Go tests across all modules (no envtest)
	@for m in $(GO_MODULES); do \
		echo ">> testing $$m"; \
		(cd $$m && go test ./...) || exit $$?; \
	done

.PHONY: test-web
test-web: ## Run web tests
	cd web && npm test --if-present

# -------- coverage --------
# Coverage profiles land in <module>/coverage/{unit,envtest,merged}.out.
# Threshold gates live in <module>/.testcoverage.yml and are enforced by
# go-test-coverage (run via `go run`, no install step required).

.PHONY: cover cover-go cover-go-integration cover-go-merge cover-go-check cover-web cover-ratchet

cover: cover-go cover-go-integration cover-go-merge cover-go-check cover-web ## Full coverage run + threshold gates

cover-go: ## Per-module Go unit coverage profiles (no envtest)
	@for m in $(GO_MODULES); do \
		echo ">> unit coverage ($$m)"; \
		mkdir -p $$m/coverage; \
		(cd $$m && go test -race -covermode=atomic -coverpkg=./... \
			-coverprofile=coverage/unit.out ./...) || exit $$?; \
	done

cover-go-integration: envtest-bin ## Per-module Go envtest coverage profiles (operator, api)
	@for m in $(GO_INTEGRATION_MODULES); do \
		echo ">> envtest coverage ($$m)"; \
		mkdir -p $$m/coverage; \
		(cd $$m && KUBEBUILDER_ASSETS="$$($(ENVTEST_BIN) use $(ENVTEST_K8S_VERSION) -p path)" \
			go test -race -tags=envtest -timeout 10m -covermode=atomic -coverpkg=./... \
			-coverprofile=coverage/envtest.out ./...) || exit $$?; \
	done

cover-go-merge: ## Merge unit + envtest profiles into coverage/merged.out per module
	@for m in $(GO_MODULES); do \
		mkdir -p $$m/coverage; \
		profiles=$$(ls $$m/coverage/unit.out $$m/coverage/envtest.out 2>/dev/null); \
		if [ -z "$$profiles" ]; then \
			echo ">> no profiles for $$m, skipping merge"; continue; \
		fi; \
		echo ">> merging coverage profiles ($$m)"; \
		go run $(GOCOVMERGE_PKG) $$profiles > $$m/coverage/merged.out || exit $$?; \
	done

cover-go-check: ## Run go-test-coverage threshold gates per module
	@for m in $(GO_MODULES); do \
		if [ ! -f $$m/.testcoverage.yml ]; then \
			echo ">> $$m has no .testcoverage.yml, skipping"; continue; \
		fi; \
		profile=$$m/coverage/merged.out; \
		[ -f $$profile ] || profile=$$m/coverage/unit.out; \
		if [ ! -f $$profile ]; then \
			echo ">> no coverage profile for $$m, skipping (run cover-go to gate it)"; continue; \
		fi; \
		echo ">> threshold check ($$m)"; \
		go run $(GO_TEST_COVERAGE_PKG) \
			--config=$$m/.testcoverage.yml \
			--profile=$$profile || exit $$?; \
	done

cover-web: ## Web coverage with vitest thresholds
	cd web && npm run test:cover --if-present

cover-ratchet: ## Print measured-vs-threshold delta per module to spot ratchet headroom
	@for m in $(GO_MODULES); do \
		profile=$$m/coverage/merged.out; \
		[ -f $$profile ] || profile=$$m/coverage/unit.out; \
		if [ ! -f $$profile ]; then \
			echo "$$m: no coverage profile (run make cover-go)"; continue; \
		fi; \
		measured=$$(cd $$m && go tool cover -func=$$(echo $$profile | sed "s|^$$m/||") | awk '/^total:/ {print $$3}'); \
		threshold=$$(awk '/^[[:space:]]*total:/ {print $$2; exit}' $$m/.testcoverage.yml 2>/dev/null); \
		printf "%-10s measured=%-7s threshold=%s\n" "$$m" "$${measured:-?}" "$${threshold:-unset}"; \
	done

# -------- integration tier (envtest) --------
ENVTEST_K8S_VERSION ?= 1.31.0
ENVTEST_BIN          := $(shell pwd)/operator/bin/setup-envtest

.PHONY: envtest-bin
envtest-bin: ## Install setup-envtest binary into operator/bin
	GOBIN=$(shell pwd)/operator/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test-integration
test-integration: envtest-bin ## Run envtest-tagged integration tests
	@for m in operator api; do \
		echo ">> integration tests ($$m)"; \
		(cd $$m && KUBEBUILDER_ASSETS="$$($(ENVTEST_BIN) use $(ENVTEST_K8S_VERSION) -p path)" \
			go test -race -tags=envtest -timeout 10m ./...) || exit $$?; \
	done

# -------- e2e tier (kind + helm + real components) --------
KIND_E2E_CLUSTER ?= kestrel-e2e
KIND_E2E_TAG     ?= e2e

.PHONY: e2e-images
e2e-images: ## Build operator/api/agent images tagged for e2e
	docker build -t kestrel-test/operator:$(KIND_E2E_TAG) -f operator/Dockerfile .
	docker build -t kestrel-test/api:$(KIND_E2E_TAG)      -f api/Dockerfile      .
	docker build -t kestrel-test/agent:$(KIND_E2E_TAG)    -f agent/Dockerfile    .

.PHONY: test-e2e
test-e2e: ## Run E2E tests (CLUSTER=kind spins an ephemeral cluster; CLUSTER=remote reuses REMOTE_KUBECONFIG)
ifeq ($(CLUSTER),remote)
	cd test/e2e && KESTREL_E2E_REUSE_CLUSTER=1 \
		KESTREL_E2E_CONTEXT=$(REMOTE_CONTEXT) KUBECONFIG=$(REMOTE_KUBECONFIG) \
		go test -tags=e2e -timeout 35m -v ./...
else
	$(MAKE) e2e-images
	cd test/e2e && KESTREL_E2E_CLUSTER=$(KIND_E2E_CLUSTER) KESTREL_E2E_TAG=$(KIND_E2E_TAG) \
		go test -tags=e2e -timeout 35m -v ./...
endif

.PHONY: test-e2e-keep
test-e2e-keep: ## Re-run E2E tests against an already-up cluster (skip create/destroy)
	cd test/e2e && KESTREL_E2E_REUSE_CLUSTER=1 KESTREL_E2E_CLUSTER=$(KIND_E2E_CLUSTER) \
		go test -tags=e2e -timeout 35m -v ./...

.PHONY: e2e-up
e2e-up: e2e-images ## Bring up the e2e kind cluster + install chart (no tests)
	./deploy/kind/e2e.sh up $(KIND_E2E_CLUSTER) $(KIND_E2E_TAG)

.PHONY: e2e-down
e2e-down: ## Tear down the e2e kind cluster
	./deploy/kind/e2e.sh down $(KIND_E2E_CLUSTER)

# -------- web e2e (Playwright) --------
# Mock mode: vite + MSW intercepting fetches. No cluster needed.
# Live mode: vite proxies fetches to a kubectl port-forward globalSetup
# spawns against the kestrel-e2e cluster. Run after `make e2e-up` and
# the Go e2e suite (which writes the admin password to test/e2e/.tmp/).

.PHONY: test-web-e2e-mock
test-web-e2e-mock: ## Playwright tests against vite + MSW mocks (no cluster)
	cd web && npm ci && npx playwright install --with-deps chromium && npm run test:e2e:mock

.PHONY: test-web-e2e-live
test-web-e2e-live: ## Playwright tests against the live e2e cluster (requires e2e-up + Go e2e)
	cd web && npm ci && npx playwright install --with-deps chromium && npm run test:e2e:live

# -------- lint --------
.PHONY: lint
lint: lint-go lint-web ## Run all linters

.PHONY: lint-go
lint-go: ## Run golangci-lint across all modules
	@for m in $(GO_MODULES); do \
		(cd $$m && golangci-lint run) || exit $$?; \
	done

.PHONY: lint-web
lint-web: ## Run web linters
	cd web && npm run lint --if-present

# -------- images --------
.PHONY: images
images: image-operator image-api image-agent ## Build all container images

image-operator: ## Build operator image
	docker build -t $(REGISTRY)/operator:$(TAG) -f operator/Dockerfile .

image-api: ## Build API image
	docker build -t $(REGISTRY)/api:$(TAG) -f api/Dockerfile .

image-agent: ## Build agent image
	docker build -t $(REGISTRY)/agent:$(TAG) -f agent/Dockerfile .

# -------- codegen --------
.PHONY: generate manifests
generate: ## Run controller-gen deepcopy generators
	cd operator && go run sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./api/...

manifests: ## Regenerate CRDs + RBAC manifests (and sync chart CRD copies)
	cd operator && go run sigs.k8s.io/controller-tools/cmd/controller-gen \
		crd:generateEmbeddedObjectMeta=true \
		rbac:roleName=manager-role \
		paths=./... \
		output:crd:artifacts:config=config/crd \
		output:rbac:artifacts:config=config/rbac
	cp operator/config/crd/gameplane.gg_*.yaml charts/kestrel/crds/

# -------- local dev cluster (kind) --------
.PHONY: dev-up dev-down dev-load dev-push dev-install
dev-up: ## Create/prepare cluster + install Kestrel (CLUSTER=kind|remote)
ifeq ($(CLUSTER),remote)
	$(MAKE) images TAG=$(TAG)
	$(MAKE) dev-push TAG=$(TAG)
	$(MAKE) modules-push MODULE_REGISTRY=$(MODULE_REGISTRY)
	$(MAKE) dev-install
else
	./deploy/kind/up.sh $(KIND_CLUSTER)
	$(MAKE) images TAG=$(TAG)
	$(MAKE) dev-load
	$(MAKE) modules-push MODULE_REGISTRY=localhost:5001
	$(MAKE) dev-install
endif

dev-load: ## Load local images into kind cluster (kind only)
	kind load docker-image $(REGISTRY)/operator:$(TAG) --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/api:$(TAG)      --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/agent:$(TAG)    --name $(KIND_CLUSTER)

dev-push: ## Push operator/api/agent images to REGISTRY (remote clusters)
	docker push $(REGISTRY)/operator:$(TAG)
	docker push $(REGISTRY)/api:$(TAG)
	docker push $(REGISTRY)/agent:$(TAG)

dev-install: ## Install Kestrel Helm chart into the selected cluster
	$(KUBECONFIG_ENV) helm upgrade --install $(CHART_RELEASE) $(CHART_DIR) \
		--namespace $(NAMESPACE) --create-namespace \
		--set image.tag=$(TAG) \
		--set image.registry=$(REGISTRY) \
		--set defaultModuleSource.url=$(MODULE_SOURCE_URL) \
		--set defaultModuleSource.insecure=$(MODULE_SOURCE_INSECURE)

dev-down: ## Tear down: delete the kind cluster, or uninstall on a remote cluster
ifeq ($(CLUSTER),remote)
	$(KUBECONFIG_ENV) helm uninstall $(CHART_RELEASE) --namespace $(NAMESPACE)
else
	kind delete cluster --name $(KIND_CLUSTER)
endif

# -------- module bundles --------
.PHONY: modules-push
MODULE_REGISTRY ?= localhost:5001
modules-push: ## Push every modules/<name> as an OCI bundle to MODULE_REGISTRY
	./modules/build.sh push --registry $(MODULE_REGISTRY) --plain-http

# -------- web dev --------
.PHONY: web-dev
web-dev: ## Run web dashboard against in-cluster API
	cd web && npm run dev

# -------- tidy --------
.PHONY: tidy
tidy: ## go mod tidy across all modules
	@for m in $(GO_MODULES); do \
		(cd $$m && go mod tidy) || exit $$?; \
	done

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin dist
	@for m in $(GO_MODULES); do rm -rf $$m/bin; done
	rm -rf web/dist web/.vite
