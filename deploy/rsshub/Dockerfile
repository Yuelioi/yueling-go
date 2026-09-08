ARG RSSHUB_BASE=ghcr.io/diygod/rsshub@sha256:3c5f0b37a54223c4735b5d6428d2886d97ae24e04297a619fb211a2bc9e174aa
FROM ${RSSHUB_BASE}
COPY patch.mjs /tmp/yueling-xiaohongshu-patch.mjs
RUN node /tmp/yueling-xiaohongshu-patch.mjs && rm /tmp/yueling-xiaohongshu-patch.mjs
