# builder
FROM --platform=$BUILDPLATFORM golang:1.24.2 AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /go/src/github.com/NVIDIA/grove
COPY ../. .

# Reference: https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope
# Docker automatically defines global scope build arguments.
ARG TARGETOS
ARG TARGETARCH

RUN make --directory=operator build-${TARGETOS}-${TARGETARCH}

# All image label keys are as per the OCI recommendations.
# Reference: https://github.com/opencontainers/image-spec/blob/main/annotations.md#pre-defined-annotation-keys

# grove-operator
FROM gcr.io/distroless/static-debian11:nonroot AS grove-operator
# Version should be passed as a build argument.
ARG VERSION

LABEL org.opencontainers.image.title="Grove Operator"
LABEL org.opencontainers.image.description="Kubernetes operator for PodGang and PodGangSet custom resources"
LABEL org.opencontainers.image.source="https://github.com/NVIDIA/grove/tree/main/operator"
LABEL org.opencontainers.image.version=${VERSION}

WORKDIR /
COPY --from=builder /go/src/github.com/NVIDIA/grove/operator/bin/grove-operator /grove-operator
ENTRYPOINT ["/grove-operator"]