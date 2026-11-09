FROM golang:1.27-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18

WORKDIR /app
COPY backend.go .
RUN go build -o backend backend.go

EXPOSE 3000

CMD ["./backend"]
