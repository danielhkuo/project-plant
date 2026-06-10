# syntax=docker/dockerfile:1
# Build context must be the repo root: the service module depends on the
# shared pkg/ module via a `replace ../../pkg` directive.
#   docker build -f services/dashboard/Dockerfile .

# ---- Builder ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY pkg/ pkg/
COPY services/dashboard/ services/dashboard/

WORKDIR /src/services/dashboard
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/dashboard ./cmd/dashboard

# Static probe for container health checks (distroless has no shell/curl).
WORKDIR /src/pkg
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/healthcheck ./cmd/healthcheck

# ---- Runner ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/dashboard /dashboard
COPY --from=build /out/healthcheck /healthcheck

EXPOSE 8081
ENTRYPOINT ["/dashboard"]
