FROM golang:1.25-alpine AS build
WORKDIR /src

COPY operator/go.mod operator/go.sum* ./operator/
WORKDIR /src/operator
RUN go mod download

WORKDIR /src
COPY operator/ ./operator/
WORKDIR /src/operator
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/manager ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
