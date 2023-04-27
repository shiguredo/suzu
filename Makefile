.PHONY: all patch test

all: patch
	go build -o bin/suzu cmd/suzu/main.go

patch:
	patch -o oggwriter.go ./pion/oggwriter.go ./patch/oggwriter.go.patch


test:
	@go test -v --race
