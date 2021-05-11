# Build the manager binary
FROM golang:1.15 as builder

# This is so that git will use SSH for this private repo; needs
# `docker build --ssh=default` to be used to initiate the build, and
# `--mount=type=ssh` below, for access to ssh-agent.
RUN git config --global url."ssh://git@github.com/".insteadOf "https://github.com/"

# This tells git to add new host keys it sees to
# the known_hosts file. This is a workaround for missing host keys in
# the base image.
ENV GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=accept-new"

# GOPRIVATE tells go mod not to use the shared proxy for modules
ENV GOPRIVATE="github.com/squaremo/fleeet"

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=ssh go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
