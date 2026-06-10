# syntax=docker/dockerfile:1
# Build context must be the repo root: the service module depends on the
# shared pkg/ module via a `replace ../../pkg` directive.
#   docker build -f services/processor/Dockerfile .

# ---- Builder ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY pkg/ pkg/
COPY services/processor/ services/processor/

WORKDIR /src/services/processor
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/processor ./cmd/processor

# Static probe for container health checks (distroless has no shell/curl).
WORKDIR /src/pkg
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/healthcheck ./cmd/healthcheck

# ---- Runner ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/processor /processor
COPY --from=build /out/healthcheck /healthcheck

# Admin server: /metrics + /health
EXPOSE 9091
ENTRYPOINT ["/processor"]
