FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o wa-agent .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/wa-agent .
COPY --from=builder /app/static ./static

EXPOSE 8080
CMD ["./wa-agent"]
