
# =====================================================================
# Stage 1 - builder: full Go toolchain + protoc. Compiles a static binary.
# This stage (~1 GB) is thrown away; only the binary is copied forward.
# =====================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache protobuf protobuf-dev git
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0

WORKDIR /src
COPY . .

# Generate gRPC/protobuf Go code from the .proto contracts.
RUN protoc -I proto \
      --go_out=. --go_opt=module=stargazer \
      --go-grpc_out=. --go-grpc_opt=module=stargazer \
      telescope/v1/telescope.proto session/v1/session.proto

# Resolve dependencies (cached between builds) and compile a static binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod tidy && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/telescope-service

# =====================================================================
# Stage 2 - runtime: tiny image with just the binary, running as non-root.
# =====================================================================
FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app
USER app
COPY --from=builder /out/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]

