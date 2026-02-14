FROM golang:1.26-alpine

WORKDIR /app
COPY backend.go .
RUN go build -o backend backend.go

EXPOSE 3000

CMD ["./backend"]
