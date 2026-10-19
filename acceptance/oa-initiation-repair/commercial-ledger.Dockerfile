ARG RUNTIME_IMAGE
FROM ${RUNTIME_IMAGE}
USER root
COPY --chown=postgres:postgres server ops backup workload-keyctl /app/
RUN chmod 0555 /app/server /app/ops /app/backup /app/workload-keyctl
USER postgres
ENTRYPOINT []
CMD ["/app/server"]
