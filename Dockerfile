# syntax=docker/dockerfile:1

# ---- builder ----
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder
WORKDIR /src
COPY . /src
ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn
RUN go build -mod=vendor -o /out/regexpl .

# ---- runtime ----
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/regexpl /usr/local/bin/regexpl
ENTRYPOINT ["/usr/local/bin/regexpl"]
CMD ["--smoke-test"]
