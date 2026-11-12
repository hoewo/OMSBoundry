FROM golang:1.26.8-alpine AS build
WORKDIR /src
COPY packages/workload-identity /src/packages/workload-identity
COPY packages/attendance-summary-contract /src/packages/attendance-summary-contract
COPY backend/services/workforce-attendance /src/backend/services/workforce-attendance
WORKDIR /src/backend/services/workforce-attendance
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/workforce-attendance ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/workforce-attendance /workforce-attendance
EXPOSE 8096
ENTRYPOINT ["/workforce-attendance"]
