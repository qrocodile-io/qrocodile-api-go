.PHONY: build test test-integration vet lint fmt fmt-check tools codegen codegen-check check tidy

build:
	go build ./...

test:
	go test -count=1 -coverprofile=coverage.out -covermode=atomic -coverpkg=github.com/qrocodile-io/qrocodile-api-go ./...

test-integration:
	QR_API_INTEGRATION_REQUIRED=true go test -count=1 ./...

vet:
	go vet ./...

lint: tools
	./tools/bin/golangci-lint run ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

# tools is a phony alias for the real, file-based tools/bin/.stamp target below, so `make lint`
# etc. only pay the `go install` cost when tools/go.mod or tools/go.sum actually changed.
tools: tools/bin/.stamp

# golangci-lint and oapi-codegen live in tools/go.mod, a separate module — see
# CONTRIBUTING.md — so their own Go-version requirements never raise the floor this module's
# consumers need.
tools/bin/.stamp: tools/go.mod tools/go.sum
	cd tools && GOBIN="$(CURDIR)/tools/bin" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
	@touch tools/bin/.stamp

codegen: tools
	go run ./cmd/codegen

codegen-check: tools
	go run ./cmd/codegen -check

tidy:
	go mod tidy
	cd tools && go mod tidy

check: fmt-check vet lint codegen-check build test
