ARG RUNTIME_IMAGE
FROM ${RUNTIME_IMAGE}
WORKDIR /app
USER root
COPY --chown=oms:oms server /app/server
RUN chmod 0555 /app/server
USER oms
ENTRYPOINT ["/app/server"]
