# -- Builder ---------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/remnacore ./cmd/remnacore

# -- Runtime ---------------------------------------------------------------
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app && adduser -u 10001 -S app -G app

COPY --from=builder /bin/remnacore /app/remnacore

WORKDIR /app

USER 10001

EXPOSE 4000 4100 4203

CMD ["./remnacore"]
