ARG RUNTIME_IMAGE
FROM ${RUNTIME_IMAGE}
COPY --chown=oms:oms server /app/server
