FROM gcr.io/distroless/static@sha256:d2b0ec3141031720cf5eedef3493b8e129bc91935a43b50562fbe5429878d96b

ARG ARCH
ARG TINI_VERSION

ADD provider /usr/local/bin/crossplane-kafka-provider

EXPOSE 8080
USER 1001
ENTRYPOINT ["crossplane-kafka-provider"]
