FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/commissary ./cmd/commissary

FROM alpine:3.20
RUN adduser -D -u 10001 commissary
COPY --from=builder /out/commissary /usr/local/bin/commissary
USER commissary
WORKDIR /home/commissary
EXPOSE 8080 9090
ENTRYPOINT ["commissary"]
