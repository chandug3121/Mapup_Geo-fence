# syntax=docker/dockerfile:1

# ---- build stage: compile a static Go binary ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage: just the binary, no shell, runs as non-root ----
FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/chandug3121/Mapup_Geo-fence" \
      org.opencontainers.image.description="Geofencing and real-time vehicle alert API"
COPY --from=build /out/server /server
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
