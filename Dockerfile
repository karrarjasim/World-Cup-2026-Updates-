# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
# CGO disabled => fully static binary, no external deps.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/mundialbot ./cmd/bot

# ---- run stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/mundialbot /app/mundialbot
# Persist state on a mounted volume so restarts never lose history.
VOLUME ["/app/data"]
ENV DATA_DIR=/app/data
USER app
ENTRYPOINT ["/app/mundialbot"]
