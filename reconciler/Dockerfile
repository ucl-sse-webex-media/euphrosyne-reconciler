FROM golang:1.21.5 AS builder

WORKDIR /workspace
COPY . .
RUN go mod download
RUN go build -o reconciler .


FROM ubuntu:jammy
WORKDIR /
COPY --from=builder /workspace/reconciler .

EXPOSE 8080
