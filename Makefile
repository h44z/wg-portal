# Go parameters
GOCMD=go
MODULENAME=github.com/h44z/wg-portal
GOFILES:=$(shell go list ./... | grep -v /vendor/)
BUILDDIR=dist
BINARIES=$(subst cmd/,,$(wildcard cmd/*))
IMAGE=h44z/wg-portal

.PHONY: all test clean phony

all: dep build

build: dep $(addsuffix -amd64,$(addprefix $(BUILDDIR)/,$(BINARIES)))
	cp scripts/wg-portal.service $(BUILDDIR)
	cp scripts/wg-portal.env $(BUILDDIR)

build-cross-plat: dep build $(addsuffix -arm,$(addprefix $(BUILDDIR)/,$(BINARIES))) $(addsuffix -arm64,$(addprefix $(BUILDDIR)/,$(BINARIES)))
	cp scripts/wg-portal.service $(BUILDDIR)
	cp scripts/wg-portal.env $(BUILDDIR)

build-docker: dep
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GOCMD) build -o $(BUILDDIR)/wgportal -ldflags "-w -s -linkmode external -extldflags \"-static\" -X github.com/h44z/wg-portal/internal/server.Version=${ENV_BUILD_IDENTIFIER}-${ENV_BUILD_VERSION}" -tags netgo cmd/wg-portal/main.go

dep:
	$(GOCMD) mod download

validate: dep
	$(GOCMD) fmt $(GOFILES)
	$(GOCMD) vet $(GOFILES)
	$(GOCMD) test -race $(GOFILES)

coverage: dep
	$(GOCMD) fmt $(GOFILES)
	$(GOCMD) test $(GOFILES) -v -coverprofile .testCoverage.txt
	$(GOCMD) tool cover -func=.testCoverage.txt  # use total:\s+\(statements\)\s+(\d+.\d+\%) as Gitlab CI regextotal:\s+\(statements\)\s+(\d+.\d+\%)

coverage-html: coverage
	$(GOCMD) tool cover -html=.testCoverage.txt

test: dep
	$(GOCMD) test $(MODULENAME)/... -v -count=1

clean:
	$(GOCMD) clean $(GOFILES)
	rm -rf .testCoverage.txt
	rm -rf $(BUILDDIR)

docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)

api-docs:
	cd internal/server; swag init --propertyStrategy pascalcase --parseDependency --parseInternal --generalInfo api.go
	$(GOCMD) fmt internal/server/docs/docs.go

$(BUILDDIR)/%-amd64: cmd/%/main.go dep phony
	GOOS=linux GOARCH=amd64 $(GOCMD) build -o $(BUILDDIR)/wgportal -ldflags "-w -s -X github.com/h44z/wg-portal/internal/server.Version=${ENV_BUILD_IDENTIFIER}-${ENV_BUILD_VERSION}" -o $@ $<

# On arch-linux install aarch64-linux-gnu-gcc to crosscompile for arm64
$(BUILDDIR)/%-arm64: cmd/%/main.go dep phony
	CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc GOOS=linux GOARCH=arm64 $(GOCMD) build -ldflags "-w -s -linkmode external -extldflags \"-static\" -X github.com/h44z/wg-portal/internal/server.Version=${ENV_BUILD_IDENTIFIER}-${ENV_BUILD_VERSION}" -o $@ $<

# On arch-linux install arm-linux-gnueabihf-gcc to crosscompile for arm
$(BUILDDIR)/%-arm: cmd/%/main.go dep phony
	CGO_ENABLED=1 CC=arm-linux-gnueabi-gcc GOOS=linux GOARCH=arm GOARM=7 $(GOCMD) build -ldflags "-w -s -linkmode external -extldflags \"-static\" -X github.com/h44z/wg-portal/internal/server.Version=${ENV_BUILD_IDENTIFIER}-${ENV_BUILD_VERSION}" -o $@ $<