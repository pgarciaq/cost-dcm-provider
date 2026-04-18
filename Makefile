BINARY_NAME := koku-cost-provider
VERSION     ?= 0.0.1-dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

run:
	go run ./cmd/$(BINARY_NAME)

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...

check: fmt vet lint test

tidy:
	go mod tidy

generate-types:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/types.gen.cfg \
		-o api/v1alpha1/types.gen.go \
		api/v1alpha1/openapi.yaml

generate-spec:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/spec.gen.cfg \
		-o api/v1alpha1/spec.gen.go \
		api/v1alpha1/openapi.yaml

generate-server:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/server/server.gen.cfg \
		-o internal/api/server/server.gen.go \
		api/v1alpha1/openapi.yaml

generate-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/client/client.gen.cfg \
		-o pkg/client/client.gen.go \
		api/v1alpha1/openapi.yaml

generate-api: generate-types generate-spec generate-server generate-client

check-generate-api: generate-api
	git diff --exit-code api/ internal/api/server/ pkg/client/ || \
		(echo "Generated files out of sync. Run 'make generate-api'." && exit 1)

container-build:
	podman build -t $(BINARY_NAME):$(VERSION) -f Containerfile .

.PHONY: build run clean fmt vet test test-cover lint check tidy \
	generate-types generate-spec generate-server generate-client \
	generate-api check-generate-api container-build
