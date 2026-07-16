.PHONY: all build clean embed test vet

GOARCHES := amd64 arm64

all: embed build

embed: bin/poqman-init bin/poqman-agent
	@for arch in $(GOARCHES); do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -o pkg/cli/bin/poqman-init-$$arch ./cmd/poqman-init/ && \
		echo "  poqman-init-$$arch built"; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -o pkg/cli/bin/poqman-agent-$$arch ./cmd/poqman-agent/ && \
		echo "  poqman-agent-$$arch built"; \
	done

bin/poqman-init:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o bin/poqman-init ./cmd/poqman-init/

bin/poqman-agent:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o bin/poqman-agent ./cmd/poqman-agent/

build:
	CGO_ENABLED=0 go build -o bin/poqman ./cmd/poqman/

test:
	CGO_ENABLED=0 go test ./... -count=1 -cover

vet:
	go vet ./...

clean:
	rm -rf bin/ pkg/cli/bin/poqman-init-* pkg/cli/bin/poqman-agent-*
