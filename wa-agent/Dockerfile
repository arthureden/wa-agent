FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go env -w GONOSUMDB=* GOFLAGS=-mod=mod
RUN go get github.com/gorilla/websocket@v1.5.1
COPY . .
RUN go build -o wa-agent .
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/wa-agent .
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./wa-agent"]
