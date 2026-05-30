# ---- build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o app

# ---- runtime stage ----
FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /app/app /app/app

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/app"]
