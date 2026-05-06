FROM gcr.io/distroless/static-debian13@sha256:47b2d72ff90843eb8a768b5c2f89b40741843b639d065b9b937b07cd59b479c6

ARG TARGETOS
ARG TARGETARCH

ADD bin/$TARGETOS\_$TARGETARCH/provider /usr/local/bin/crossplane-kafka-provider

USER 65532
ENTRYPOINT ["crossplane-kafka-provider"]