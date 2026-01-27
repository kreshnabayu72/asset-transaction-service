FROM golang:1.25.5 AS builder

WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates openssl
COPY certs/CA.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

ENV GOPRIVATE=gitea.tagsamurai.local/BETS-V2/ts-utils/v2
ARG USER
ARG PAT
RUN git config --global url."https://${USER}:${PAT}@gitea.tagsamurai.local/".insteadOf "https://gitea.tagsamurai.local/"
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .

FROM alpine:latest
WORKDIR /tagsamurai/api/analytics-reporting
COPY --from=builder /app/main .
CMD ["./main"]