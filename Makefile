# Go parameters
GOCMD=go
MODULENAME=github.com/h44z/wg-portal
GOFILES:=$(shell go list ./... | grep -v /vendor/)
BUILDDIR=dist
BINARIES=$(subst cmd/,,$(wildcard cmd/*))

.PHONY: all test clean phony

all: dep test build

build: dep $(addprefix $(BUILDDIR)/,$(BINARIES))
	cp -r assets $(BUILDDIR)

dep:
	$(GOCMD) mod download

validate:
	$(GOCMD) fmt $(GOFILES)
	$(GOCMD) vet $(GOFILES)
	$(GOCMD) test -race $(GOFILES)

coverage:
	$(GOCMD) fmt $(GOFILES)
	$(GOCMD) test $(GOFILES) -v -coverprofile .testCoverage.txt
	$(GOCMD) tool cover -func=.testCoverage.txt  # use total:\s+\(statements\)\s+(\d+.\d+\%) as Gitlab CI regextotal:\s+\(statements\)\s+(\d+.\d+\%)

coverage-html: coverage
	$(GOCMD) tool cover -html=.testCoverage.txt

test:
	$(GOCMD) test $(MODULENAME)/... -v -count=1

clean:
	$(GOCMD) clean $(GOFILES)
	rm -rf .testCoverage.txt
	rm -rf $(BUILDDIR)

$(BUILDDIR)/%: cmd/%/main.go dep phony
	$(GOCMD) build -o $@ $<