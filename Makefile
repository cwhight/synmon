.PHONY: build run test lint docker-build docker-run clean sync-standards

BINARY := synmon
DOCKER_TAG := synmon:latest
STANDARDS := go architecture observability concurrency service-lifecycle database feature-flags testing

build:
	go build -o $(BINARY) ./cmd/synmon

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(DOCKER_TAG) .

docker-run: docker-build
	docker run --rm \
		-e TRACETEST_API_ENDPOINT=http://host.docker.internal:11633 \
		-e TEST_INTERVAL=30s \
		-e TESTS_FILE=/etc/synmon/tests.yaml \
		-p 2112:2112 \
		$(DOCKER_TAG)

compose-up:
	docker compose up -d

compose-down:
	docker compose down

sync-standards:
	@mkdir -p standards
	@for f in $(STANDARDS); do \
		gh api repos/cwhight/go-standards/contents/$$f.md --jq '.content' | base64 -d > standards/$$f.md; \
	done
	@echo "Standards synced from cwhight/go-standards"

clean:
	rm -f $(BINARY)
