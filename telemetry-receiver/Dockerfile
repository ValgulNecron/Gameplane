FROM golang:1.25-alpine AS build
WORKDIR /src

COPY telemetry-receiver/go.mod telemetry-receiver/go.sum* ./telemetry-receiver/
WORKDIR /src/telemetry-receiver
RUN go mod download

WORKDIR /src
COPY telemetry-receiver/ ./telemetry-receiver/
WORKDIR /src/telemetry-receiver

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/telemetry-receiver .

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/telemetry-receiver /telemetry-receiver
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/telemetry-receiver"]
