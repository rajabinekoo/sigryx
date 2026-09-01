############################
# Build stage
############################
FROM golang:1.26-alpine AS build

RUN apk add --no-cache ca-certificates tzdata libsodium-dev pkgconf

ARG GOPROXY=https://goproxy.io|direct

ENV CGO_ENABLED=1 \
    GOFLAGS=-trimpath \
    GOPROXY=${GOPROXY} \
    GO111MODULE=on

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/sigryx ./services/sigryx/cmd

############################
# Minimal runtime stage for the static Go binary.
############################
FROM scratch AS runtime

COPY --from=build /out/sigryx /usr/local/bin/sigryx

EXPOSE 8080 50051

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/sigryx"]
