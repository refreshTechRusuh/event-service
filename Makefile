run:
	go run cmd/main.go
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api cmd/main.go
test:
	go test -v ./...
unit-test-coverage:
	go test -v -covermode=count $$(go list ./... | grep -v '/test/') -coverprofile=coverage.cov
	go tool cover -func=coverage.cov
mod-tidy:
	go mod tidy
golint:
	golangci-lint run --timeout 5m
gen-mock-event-inbound:
	mockgen -package=mock_inbound -destination=_mock/event/port/inbound/event_mock.go -source=internal/event/port/inbound/event.go
gen-mock-event-outbound:
	mockgen -package=mock_outbound -destination=_mock/event/port/outbound/event_repo_mock.go -source=internal/event/port/outbound/event_repo.go
gen-mocks: gen-mock-event-inbound gen-mock-event-outbound
docker-build: build
	docker build -f Dockerfile -t event-service:latest .

scan:
	@echo "Running security scan"
	gosec ./...
