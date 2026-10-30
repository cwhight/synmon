FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /synmon ./cmd/synmon

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /synmon /usr/local/bin/synmon

# Default tests file location (mount your own via volume/configmap)
COPY examples/tests.yaml /etc/synmon/tests.yaml

EXPOSE 2112

ENTRYPOINT ["synmon"]
