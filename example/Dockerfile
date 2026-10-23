FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c

WORKDIR /app
COPY backend.go .
RUN go build -o backend backend.go

EXPOSE 3000

CMD ["./backend"]
