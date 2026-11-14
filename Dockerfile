# syntax=docker/dockerfile:1.7
FROM golang:1.27.3 AS builder

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates openssl git \
	&& rm -rf /var/lib/apt/lists/*
COPY certs/CA.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

ENV GOPRIVATE=gitea.tagsamurai.local/*
ARG USER
ARG PAT
RUN git config --global url."https://${USER}:${PAT}@gitea.tagsamurai.local/".insteadOf "https://gitea.tagsamurai.local/"
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/main .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /tagsamurai/api/asset-transaction
COPY --from=builder /app/main .
CMD ["./main"]