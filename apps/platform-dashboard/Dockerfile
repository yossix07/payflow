# Build Stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
# Initialize go module if not exists (though we will create one)
RUN go mod init platform-dashboard || true
RUN go build -o main .

# Run Stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
COPY static ./static
RUN chmod +x ./main
EXPOSE 8080
CMD ["./main"]
