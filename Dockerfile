FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o prism .

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

# Fixed UID/GID so bind-mounted volumes can be chowned predictably.
RUN apk --no-cache add ca-certificates \
    && addgroup -S -g 1000 prism && adduser -S -u 1000 -G prism prism \
    && mkdir /data && chown prism:prism /data

WORKDIR /app
COPY --from=builder /app/prism .

USER prism

EXPOSE 8000 8080

CMD ["./prism"]
