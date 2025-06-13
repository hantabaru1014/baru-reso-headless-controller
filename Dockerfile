FROM --platform=$BUILDPLATFORM golang:1.24 AS build
ARG TARGETARCH

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

RUN --mount=type=bind,source=.,target=.,ro \
    GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /go/bin/app cmd/server/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/app /
USER nonroot
CMD ["/app"]