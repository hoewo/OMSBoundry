FROM golang:1.26.0-alpine AS build
WORKDIR /src
COPY packages/workload-identity ./packages/workload-identity
COPY backend/services/project-workspace/go.mod backend/services/project-workspace/go.sum ./backend/services/project-workspace/
RUN cd backend/services/project-workspace && go mod download
COPY backend/services/project-workspace ./backend/services/project-workspace
RUN cd backend/services/project-workspace && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/project-workspace ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/project-workspace /usr/local/bin/project-workspace
USER app
EXPOSE 8086
ENTRYPOINT ["project-workspace"]
