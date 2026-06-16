FROM registry.access.redhat.com/ubi9/go-toolset:1.25.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.0.1-dev
USER root
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags "-X main.version=${VERSION}" -o koku-cost-provider ./cmd/koku-cost-provider

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /app

COPY --from=builder /app/koku-cost-provider .

EXPOSE 8080

ENTRYPOINT ["./koku-cost-provider"]
