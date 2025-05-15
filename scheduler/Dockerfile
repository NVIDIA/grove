# builder
FROM --platform=$BUILDPLATFORM golang:1.24.2 AS builder
WORKDIR /go/src/github.com/NVIDIA/grove
COPY ../. .

# Reference: https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope
# Docker automatically defines global scope build arguments.
ARG TARGETOS
ARG TARGETARCH

RUN make --directory=scheduler build-${TARGETOS}-${TARGETARCH}

# All image label keys are as per the OCI recommendations.
# Reference: https://github.com/opencontainers/image-spec/blob/main/annotations.md#pre-defined-annotation-keys

# scheduler
FROM gcr.io/distroless/static-debian11:nonroot AS kube-scheduler
# Version should be passed as a build argument.
ARG VERSION

LABEL org.opencontainers.image.title="Grove kube-scheduler"
LABEL org.opencontainers.image.description="Kubernetes scheduler which supports network optimized Gang scheduling"
LABEL org.opencontainers.image.source="https://github.com/NVIDIA/grove/tree/main/scheduler"
LABEL org.opencontainers.image.version=${VERSION}

WORKDIR /bin
COPY --from=builder /go/src/github.com/NVIDIA/grove/scheduler/bin/kube-scheduler /bin/kube-scheduler
ENTRYPOINT ["/bin/kube-scheduler"]