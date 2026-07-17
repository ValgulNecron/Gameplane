# Gameplane dashboard image: build the Vite SPA, serve it with nginx.
#
# Build context is the repo root (matches the other components: the
# publish/release workflows pass `context: .` and `file: web/Dockerfile`).
# Everything this stage needs lives under web/, so only web/ is copied in.

# ---- build the SPA ----
FROM node:22-bookworm-slim AS build
WORKDIR /src/web

# Install deps first (cached until the lockfile changes), then build.
COPY web/package.json web/package-lock.json* ./
RUN npm ci

COPY web/ ./
# Vite emits to web/dist. tsc + vite build run via `npm run build`.
RUN npm run build

# ---- serve with a non-root nginx ----
# nginx-unprivileged listens on 8080 and runs as uid 101 (nginx), so it needs
# no root and satisfies runAsNonRoot/readOnlyRootFilesystem in the chart.
FROM nginxinc/nginx-unprivileged:1.27-alpine

# API_UPSTREAM is substituted into the config at container start by the base
# image's envsubst step (it templates /etc/nginx/templates/*.template into
# /etc/nginx/conf.d/). The chart sets it to the in-cluster API service; this
# default keeps a bare `docker run` from failing to start.
ENV API_UPSTREAM=http://gameplane-api.gameplane-system.svc.cluster.local

# Replace the base image's default server block with ours.
RUN rm -f /etc/nginx/conf.d/default.conf
COPY web/nginx.conf.template /etc/nginx/templates/gameplane.conf.template

# Static assets.
COPY --from=build /src/web/dist /usr/share/nginx/html

EXPOSE 8080
