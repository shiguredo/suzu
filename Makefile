.PHONY: all

all:
	go build -o bin/suzu cmd/suzu/main.go

init:
	curl -LO https://raw.githubusercontent.com/pion/webrtc/master/pkg/media/oggwriter/oggwriter.go
	patch < oggwriter.go.patch

test:
	@go test -v -short --race
