FROM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523

WORKDIR /app
COPY backend.go .
RUN go build -o backend backend.go

EXPOSE 3000

CMD ["./backend"]
