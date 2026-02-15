ARG BUILDER_IMAGE=ghcr.io/geonet/base-images/golang:1.23-alpine3.21
ARG RUNNER_IMAGE=ghcr.io/geonet/base-images/alpine:3.21
ARG RUN_USER=nobody

FROM ${BUILDER_IMAGE} as builder
RUN apk add --update ca-certificates tzdata
ARG BUILD=nema-mar-app
ARG GIT_COMMIT_SHA
COPY ./ /repo
WORKDIR /repo
ENV GOBIN /repo/gobin
ENV GOPATH /usr/src/go
ENV GOFLAGS -mod=vendor
ENV CGO_ENABLED 0
ENV GOOS linux
ENV GOARCH amd64
RUN echo 'nobody:x:65534:65534:Nobody:/:\' > /passwd
RUN go install -a -installsuffix cgo \
    -ldflags "-X main.Prefix=${BUILD}/${GIT_COMMIT_SHA}" \
    /repo/cmd/${BUILD}

FROM ${RUNNER_IMAGE}
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /passwd /etc/passwd
ENV TZ Pacific/Auckland
ENV GODEBUG madvdontneed=1

COPY --from=builder /repo/gobin/nema-mar-app /usr/local/bin/nema-mar-app
COPY --from=builder /repo/cmd/nema-mar-app/templates /app/templates
COPY --from=builder /repo/schema /app/schema

ARG RUN_USER=nobody
USER ${RUN_USER}
WORKDIR /app
CMD ["/usr/local/bin/nema-mar-app"]
