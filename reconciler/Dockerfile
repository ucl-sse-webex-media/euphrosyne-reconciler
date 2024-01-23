FROM golang:1.21.6 AS builder

WORKDIR /workspace
COPY . .
RUN go mod download
RUN go build -o reconciler .

FROM arm64v8/ubuntu:jammy
WORKDIR /
COPY --from=builder /workspace/reconciler .

EXPOSE 8080
