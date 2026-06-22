# Stage 1: build the binary (DB migrations are embedded via go:embed)
FROM golang:1.26-alpine AS builder
WORKDIR /src
# warm the module cache before copying the full source so code changes
# do not invalidate this layer
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/billing .

# Stage 2: lightweight runtime with only the binary
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /out/billing .
RUN chmod +x ./billing
# BILLING_HTTP_ADDR must be set to ":4000" (or "0.0.0.0:4000") in the container;
# the default "localhost:4000" only binds the loopback interface.
EXPOSE 4000
CMD ["./billing"]
