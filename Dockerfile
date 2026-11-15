FROM golang:1.27-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o prism .

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# Fixed UID/GID so bind-mounted volumes can be chowned predictably.
RUN apk --no-cache add ca-certificates \
    && addgroup -S -g 1000 prism && adduser -S -u 1000 -G prism prism \
    && mkdir /data && chown prism:prism /data

WORKDIR /app
COPY --from=builder /app/prism .

USER prism

EXPOSE 8000 8080

CMD ["./prism"]
