FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o prism .

FROM alpine:3.24

# Fixed UID/GID so bind-mounted volumes can be chowned predictably.
RUN apk --no-cache add ca-certificates \
    && addgroup -S -g 1000 prism && adduser -S -u 1000 -G prism prism \
    && mkdir /data && chown prism:prism /data

WORKDIR /app
COPY --from=builder /app/prism .

USER prism

EXPOSE 8000 8080

CMD ["./prism"]
