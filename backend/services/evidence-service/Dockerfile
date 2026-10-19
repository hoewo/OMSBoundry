FROM golang:1.26.0-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILT_AT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN ldflags="-s -w -X omsbizfoundry/evidence-service/internal/buildinfo.Version=${VERSION} -X omsbizfoundry/evidence-service/internal/buildinfo.Commit=${COMMIT} -X omsbizfoundry/evidence-service/internal/buildinfo.BuiltAt=${BUILT_AT}" && \
    CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o /out/evidence-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o /out/evidence-ops ./cmd/ops && \
    CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o /out/evidence-backup ./cmd/backup

FROM postgres:18.6-alpine
WORKDIR /app
COPY --from=build /out/evidence-server /usr/local/bin/evidence-server
COPY --from=build /out/evidence-ops /usr/local/bin/evidence-ops
COPY --from=build /out/evidence-backup /usr/local/bin/evidence-backup
RUN mkdir -p /var/lib/evidence/objects && chown -R postgres:postgres /var/lib/evidence
VOLUME ["/var/lib/evidence"]
USER postgres
EXPOSE 8085
ENTRYPOINT ["evidence-server"]
CMD []
