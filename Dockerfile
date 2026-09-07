# syntax=docker/dockerfile:1
FROM golang:1.27 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/kpulse ./cmd/kpulse

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/kpulse /usr/local/bin/kpulse
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kpulse"]
