.PHONY: proto tidy fmt vet test up down logs

PROTO_FILES = telescope/v1/telescope.proto session/v1/session.proto

## Generate Go code from the .proto files into ./gen
proto:
	protoc -I proto --go_out=. --go_opt=module=stargazer --go-grpc_out=. --go-grpc_opt=module=stargazer $(PROTO_FILES)

## Resolve dependencies (creates go.sum - commit it)
tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

## Start the whole system
up:
	docker compose up --build

## Stop everything and delete database volumes
down:
	docker compose down -v

logs:
	docker compose logs -f
