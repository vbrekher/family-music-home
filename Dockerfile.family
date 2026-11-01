# syntax=docker/dockerfile:1
FROM node:22.22.1-alpine@sha256:8094c002d08262dba12645a3b4a15cd6cd627d30bc782f53229a2ec13ee22a00 AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ./
COPY ui/vendor/ ./vendor/
COPY ui/bin/ ./bin/
RUN --mount=type=cache,target=/root/.npm npm ci --legacy-peer-deps --fetch-retries=5
COPY ui/ ./
RUN npm run build

FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build
RUN apk add --no-cache gcc musl-dev zlib-dev git
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=ui /ui/build ./ui/build
ARG GIT_SHA=development
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=1 go build -tags netgo,sqlite_fts5 -ldflags="-s -w -X github.com/navidrome/navidrome/consts.gitSha=${GIT_SHA} -X github.com/navidrome/navidrome/consts.gitTag=1.0.0" -o /out/navidrome .

FROM deluan/navidrome@sha256:38dc2727bfcfd5ede290f8ada114fc90368146f265ae4701ddddbcbe2a44ee52
RUN apk upgrade --no-cache && apk add --no-cache python3
COPY integrations/bunny/requirements.txt /tmp/bunny-requirements.txt
RUN apk add --no-cache --virtual .fmh-python-build py3-pip \
    && python3 -m venv /opt/bunny-venv \
    && /opt/bunny-venv/bin/pip install --no-cache-dir -r /tmp/bunny-requirements.txt \
    && /opt/bunny-venv/bin/pip uninstall --yes pip \
    && apk del .fmh-python-build
ENV PATH="/opt/bunny-venv/bin:${PATH}"
ARG GIT_SHA=development
LABEL org.opencontainers.image.source="https://github.com/aitishnyk/family-music-home" \
      org.opencontainers.image.revision="${GIT_SHA}" \
      org.opencontainers.image.licenses="GPL-3.0" \
      org.opencontainers.image.title="Family Music Home"
COPY --from=build /out/navidrome /app/navidrome
COPY integrations/selfhost/ /app/selfhost/
COPY integrations/bunny/ /app/bunny/
ENV ND_APPNAME="Family Music Home" FMH_INSTALL_MODE=PUBLIC_SELF_HOST ND_ENABLESHARING=false ND_ENABLEEXTERNALSERVICES=false ND_ENABLEINSIGHTSCOLLECTOR=false ND_ENABLEGRAVATAR=false ND_DEFAULTTHEME=Nord ND_LOGLEVEL=warn
ENTRYPOINT ["python3", "/app/bunny/runtime.py", "/app/navidrome"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD curl --fail --silent http://127.0.0.1:4533/ping || exit 1
