ARG PG_CONTAINER_VERSION=16.10

FROM postgres:${PG_CONTAINER_VERSION}-alpine AS builder

ARG SCWS_VERSION=1.2.3
ARG ZHPARSER_VERSION=v2.3
ARG ZHPARSER_COMMIT=dd292fd

RUN apk add --no-cache \
    autoconf \
    automake \
    build-base \
    git \
    libtool \
    linux-headers \
    m4 \
    postgresql-dev

RUN set -eux; \
    git clone --branch "${SCWS_VERSION}" --single-branch --depth 1 https://github.com/hightman/scws.git /build/scws; \
    cd /build/scws; \
    touch README; \
    aclocal; \
    autoconf; \
    autoheader; \
    libtoolize; \
    automake --add-missing; \
    ./configure; \
    make -j"$(nproc)"; \
    make install

RUN set -eux; \
    git clone --branch "${ZHPARSER_VERSION}" --single-branch --depth 1 https://github.com/amutu/zhparser.git /build/zhparser; \
    cd /build/zhparser; \
    test "$(git rev-parse --short=7 HEAD)" = "${ZHPARSER_COMMIT}"; \
    make with_llvm=no -j"$(nproc)"; \
    make with_llvm=no install

FROM postgres:${PG_CONTAINER_VERSION}-alpine

ENV LANG=zh_CN.UTF-8

COPY --from=builder /usr/local/lib/postgresql/zhparser.so /usr/local/lib/postgresql/
COPY --from=builder /usr/local/lib/libscws.* /usr/local/lib/
COPY --from=builder /usr/local/share/postgresql/extension/zhparser* /usr/local/share/postgresql/extension/
COPY --from=builder /usr/local/share/postgresql/tsearch_data/*.utf8.* /usr/local/share/postgresql/tsearch_data/
