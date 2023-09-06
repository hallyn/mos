include subs.mk

BUILD_TAGS = containers_image_openpgp
TOOLSDIR := $(shell pwd)/hack/tools
PATH := bin:$(TOOLSDIR)/bin:$(PATH)
# OCI registry
ZOT := $(TOOLSDIR)/bin/zot
# OCI registry clients
ORAS := $(TOOLSDIR)/bin/oras
ORAS_VERSION := 1.0.0-rc.1
REGCTL := $(TOOLSDIR)/bin/regctl
REGCTL_VERSION := 0.5.0
TOPDIR := $(shell git rev-parse --show-toplevel)
BOOTKIT_VERSION ?= "v0.0.15.230901"

MAIN_VERSION ?= $(shell git describe --always --dirty || echo no-git)
ifeq ($(MAIN_VERSION),$(filter $(MAIN_VERSION), "", no-git))
$(error "Bad value for MAIN_VERSION: '$(MAIN_VERSION)'")
endif

GO_SRC=$(shell find cmd pkg  -name "*.go")

all: mosctl mosb trust bkcust $(ZOT) $(ORAS) $(REGCTL)

VERSION_LDFLAGS=-X github.com/project-machine/mos/pkg/mosconfig.Version=$(MAIN_VERSION) \
	-X github.com/project-machine/mos/pkg/trust.Version=$(MAIN_VERSION) \
	-X github.com/project-machine/mos/pkg/mosconfig.LayerVersion=0.0.1 \
	-X github.com/project-machine/mos/pkg/trust.BootkitVersion=$(BOOTKIT_VERSION)

mosctl: .made-gofmt $(GO_SRC)
	go build -tags "$(BUILD_TAGS)" -ldflags "-s -w $(VERSION_LDFLAGS)" ./cmd/mosctl

mosb: .made-gofmt $(GO_SRC)
	go build -tags "$(BUILD_TAGS)" -ldflags "-s -w $(VERSION_LDFLAGS)" ./cmd/mosb

trust: .made-gofmt $(GO_SRC)
	go build -tags "$(BUILD_TAGS)" -ldflags "-s -w $(VERSION_LDFLAGS)" ./cmd/trust

bkcust: .made-gofmt $(GO_SRC)
	go build -tags "$(BUILD_TAGS)" -ldflags "-s -w $(VERSION_LDFLAGS)" ./cmd/bkcust

$(ZOT):
	mkdir -p $(TOOLSDIR)/bin
	curl -Lo $(ZOT) https://github.com/project-zot/zot/releases/download/v$(ZOT_VERSION)/zot-linux-amd64-minimal
	chmod +x $(ZOT)

$(ORAS):
	mkdir -p $(TOOLSDIR)/bin
	curl -Lo oras.tar.gz https://github.com/oras-project/oras/releases/download/v$(ORAS_VERSION)/oras_$(ORAS_VERSION)_linux_amd64.tar.gz
	tar xvzf oras.tar.gz -C $(TOOLSDIR)/bin oras
	rm oras.tar.gz

$(REGCTL):
	mkdir -p $(TOOLSDIR)/bin
	curl -Lo $(REGCTL) https://github.com/regclient/regclient/releases/download/v$(REGCTL_VERSION)/regctl-linux-amd64
	chmod +x $(REGCTL)

.PHONY: gofmt
gofmt: .made-gofmt

.made-gofmt: $(GO_SRC)
	@o=$$(gofmt -l -w cmd pkg 2>&1) || \
	  { r=$$?; echo "gofmt failed [$$r]: $$o" 1>&2; exit $$r; }; \
	  [ -z "$$o" ] || { echo "gofmt made changes: $$o" 1>&2; exit 1; }
	@touch $@

deps: mosctl mosb trust $(ORAS) $(REGCTL) $(ZOT)

STACKER_OPTS = --layer-type=squashfs $(STACKER_SUBS)

.PHONY: layers
layers: mosctl
	stacker build $(STACKER_OPTS) --stacker-file layers/provision/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/install/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/bootkit/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/build-krd/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/kernel/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/minbase/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/mos/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/ovmf/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/rootfs/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/shim/stacker.yaml
	stacker build $(STACKER_OPTS) --stacker-file layers/stubby/stacker.yaml


.PHONY: test
test: deps
	bats tests/install.bats
	bats tests/rfs.bats
	bats tests/activate.bats
	bats tests/update.bats
	bats tests/mount.bats
	bats tests/keyset.bats
	bats tests/project.bats
	bats tests/sudi.bats

# the trust testcases only, for running on amd64.  We need an arm64
# runner capable of doing nested virt if we're going to have github
# actions run the mos tests for arm64, and we don't have that.  Yet.
.PHONY: test-trust
test-trust: trust
	bats tests/keyset.bats
	bats tests/project.bats
	bats tests/sudi.bats

clean:
	rm -f mosb mosctl trust
	rm -rf $(TOOLSDIR)
	stacker clean
