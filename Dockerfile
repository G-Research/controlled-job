# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
ARG BASE_IMAGE=gcr.io/distroless/static
ARG TAG=nonroot
FROM $BASE_IMAGE:$TAG

WORKDIR /
USER 1000:1000
COPY bin/manager .

ENTRYPOINT ["/manager"]