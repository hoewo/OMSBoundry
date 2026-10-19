FROM golang:1.26.0-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN GOWORK=off go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILT_AT=unknown
RUN --mount=type=cache,target=/root/.cache/go-build set -eux; \
    ldflags="-s -w -X omsbizfoundry/party-directory/internal/buildinfo.Version=${VERSION} -X omsbizfoundry/party-directory/internal/buildinfo.Commit=${COMMIT} -X omsbizfoundry/party-directory/internal/buildinfo.BuiltAt=${BUILT_AT}"; \
    CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags "$ldflags" -o /out/party-directory-server ./cmd/server; \
    CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags "$ldflags" -o /out/party-directory-ops ./cmd/ops; \
    CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags "$ldflags" -o /out/party-directory-backup ./cmd/backup

FROM postgres:18.6-alpine
COPY --from=build --chown=postgres:postgres /out/party-directory-server /usr/local/bin/
COPY --from=build --chown=postgres:postgres /out/party-directory-ops /usr/local/bin/
COPY --from=build --chown=postgres:postgres /out/party-directory-backup /usr/local/bin/
USER postgres
EXPOSE 8090
ENTRYPOINT ["party-directory-server"]
