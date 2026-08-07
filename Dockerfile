# Multi-stage build producing a minimal distroless image for Cloud Run.
# Build target selects which binary (api or worker) to package - both live in this one repo.
ARG BINARY=api

FROM golang:1.25-bookworm AS build
ARG BINARY
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${BINARY}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
