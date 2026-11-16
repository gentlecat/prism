FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280

WORKDIR /app
COPY backend.go .
RUN go build -o backend backend.go

EXPOSE 3000

CMD ["./backend"]
