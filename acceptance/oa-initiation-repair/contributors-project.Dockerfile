ARG RUNTIME_IMAGE
FROM ${RUNTIME_IMAGE}
WORKDIR /app
USER root
COPY --chown=postgres:postgres server ops workload-keyctl /app/
RUN chmod 0555 /app/server /app/ops /app/workload-keyctl
USER postgres
ENTRYPOINT ["/app/server"]
