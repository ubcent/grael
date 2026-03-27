GOCACHE := $(CURDIR)/.gocache
BINARY := bin/grael
DEMO_BINARY := bin/grael-demo
PROTOC := protoc
PROTO_DIR := proto
PROTO_GO_OUT := .

.PHONY: build build-demo proto test run clean example-start example-living-dag-start example-living-dag-ops-start example-core-demo-start demo-ui example-status example-events example-snapshot

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o $(BINARY) ./cmd/grael

build-demo:
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -o $(DEMO_BINARY) ./cmd/grael-demo

proto:
	PATH="$(PATH):$(shell go env GOPATH)/bin" $(PROTOC) \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_GO_OUT) \
		--go_opt=module=grael \
		--go-grpc_out=$(PROTO_GO_OUT) \
		--go-grpc_opt=module=grael \
		$(PROTO_DIR)/grael.proto

test:
	GOCACHE=$(GOCACHE) go test ./...

run: build
	./$(BINARY)

clean:
	rm -rf bin .gocache

example-start: build
	./$(BINARY) start -workflow examples/workflows/linear-noop.json -demo-worker

example-living-dag-start: build
	./$(BINARY) start -workflow examples/workflows/living-dag.json -demo-worker

example-living-dag-ops-start: build
	./$(BINARY) start -workflow examples/workflows/living-dag-ops.json -demo-worker

example-core-demo-start: build
	./$(BINARY) start -workflow examples/workflows/core-demo.json -demo-worker

demo-ui: build-demo
	./$(DEMO_BINARY)

example-status: build
	@echo "usage: make example-status RUN_ID=<run-id>"
	@test -n "$(RUN_ID)" && ./$(BINARY) status -run-id $(RUN_ID)

example-events: build
	@echo "usage: make example-events RUN_ID=<run-id>"
	@test -n "$(RUN_ID)" && ./$(BINARY) events -run-id $(RUN_ID)

example-snapshot: build
	@echo "usage: make example-snapshot RUN_ID=<run-id>"
	@test -n "$(RUN_ID)" && ./$(BINARY) snapshot -run-id $(RUN_ID)
