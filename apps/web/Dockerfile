# Dockerfile for apps/web. Multi-stage: build the Vue app with
# Vite, then serve the static dist with nginx. The image
# expects the repo root as build context so it can install
# workspace dependencies and run `pnpm build` for the web
# workspace.
#
# Build: docker build -t ocpp-web -f apps/web/Dockerfile .

# syntax=docker/dockerfile:1.6

FROM node:20-alpine AS builder
WORKDIR /src

# Install pnpm 9.15.4 directly via npm. corepack's bundled
# pnpm 11.x crashes on Node 20 (ERR_UNKNOWN_BUILTIN_MODULE), so
# we pin a known-good 9.x release. 9.x is the last major that
# supports Node 20 reliably.
RUN npm install -g pnpm@9.15.4 && pnpm --version

# Install workspace deps first for layer caching.
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml* ./
COPY apps/web/package.json ./apps/web/package.json
RUN --mount=type=cache,target=/pnpm/store \
    pnpm install --frozen-lockfile=false --filter ./apps/web...

COPY apps/web/ ./apps/web/
WORKDIR /src/apps/web
RUN --mount=type=cache,target=/pnpm/store \
    pnpm run build

FROM nginx:1.27-alpine
COPY --from=builder /src/apps/web/dist /usr/share/nginx/html
COPY docker/web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
HEALTHCHECK CMD wget -qO- http://localhost/ >/dev/null 2>&1 || exit 1
