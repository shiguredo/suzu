.PHONY: all patch test

LIST := $(GOOS) $(GOARCH)
SUFFIX := $(shell printf "_%s" $(LIST))

all: patch
	go build -o bin/suzu cmd/suzu/main.go

patch:
	patch -o oggwriter.go ./_third_party/pion/oggwriter.go ./patch/oggwriter.go.patch
	patch -o util.go ./_third_party/pion/util.go ./patch/util.go.patch


test:
	@go test -v --race

release: patch
ifeq ($(SUFFIX),_)
	CGO_ENABLED=0 go build -o dist/suzu cmd/suzu/main.go
else
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o dist/suzu$(SUFFIX) cmd/suzu/main.go
endif