FROM golang:1.26 AS build
COPY bar/ ./bar/
COPY foo/ ./foo/
RUN echo build

FROM scratch
COPY --from=build /out/foo /foo
