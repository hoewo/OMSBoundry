# syntax=docker/dockerfile:1.7
ARG GO_IMAGE=golang:1.26.0-alpine
ARG RUNTIME_IMAGE=postgres:18.6-alpine

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG APP
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY packages/workload-identity ./packages/workload-identity
COPY backend/services/${APP}/go.mod backend/services/${APP}/go.sum ./backend/services/${APP}/
RUN cd backend/services/${APP} && go mod download
COPY backend/services/${APP}/ ./backend/services/${APP}/
RUN cd backend/services/${APP} \
    && GOWORK=off CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server \
    && GOWORK=off CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -trimpath -ldflags='-s -w' -o /out/ops ./cmd/ops

FROM ${RUNTIME_IMAGE}
RUN mkdir -p /app /data/evidence /data/contract-evidence \
    && chown -R postgres:postgres /app /data/evidence /data/contract-evidence
COPY --from=build /out/server /out/ops /app/
USER postgres
ENTRYPOINT ["/app/server"]
