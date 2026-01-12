FROM gcr.io/distroless/static-debian13@sha256:30632f208e79a7564f70a60758e658d050d1747495762c2a7af0e56b6a47a2b8

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/provider /usr/local/bin/crossplane-kafka-provider

USER 65532
ENTRYPOINT ["crossplane-kafka-provider"]