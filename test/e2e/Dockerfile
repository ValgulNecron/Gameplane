# Image for the e2e game-bot probe (test/e2e/cmd/gameprobe).
#
# Built only by the e2e game-bot CI job and loaded straight into kind; it is
# never published. Running the protocol bot from inside the cluster lets that
# job dial the game Service directly, exercising the same network path a real
# in-cluster client uses instead of a `kubectl port-forward` tunnel.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY test/e2e/ ./
# gameprobe imports only the stdlib plus this module's own bot packages, so it
# builds against test/e2e's go.mod alone — no need to drag in the go.work
# workspace (and its k8s dependencies) just to compile a TCP client.
ENV GOWORK=off
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w" -o /out/gameprobe ./cmd/gameprobe

FROM gcr.io/distroless/static:nonroot
USER 65532:65532
COPY --from=build /out/gameprobe /gameprobe
ENTRYPOINT ["/gameprobe"]
