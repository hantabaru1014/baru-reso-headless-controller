FROM node:23-slim AS front-build
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable
COPY . /app
WORKDIR /app
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile
RUN pnpm run build

FROM --platform=$BUILDPLATFORM golang:1.23 AS build
ARG TARGETARCH

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=front-build /app/front/dist ./front/dist

RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /go/bin/app cmd/server/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/app /
USER nonroot
CMD ["/app"]