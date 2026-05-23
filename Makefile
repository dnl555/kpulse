BINARY := kpulse
PKG    := github.com/dnl555/kpulse
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint image render-yaml clean

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/kpulse

test:
	go test ./... -race -count=1

lint:
	golangci-lint run

image:
	docker build --build-arg VERSION=$(VERSION) -t kpulse:$(VERSION) .

render-yaml:
	@VERSION=$(VERSION) ./deploy/render.sh > deploy/kpulse.yaml

clean:
	rm -rf bin/ dist/
