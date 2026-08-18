# syntax=docker/dockerfile:1

# ---------------------------------------------------------------- build stage
FROM node:24-alpine AS build

WORKDIR /app

# Lockfile first, so source edits do not re-install dependencies.
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# --------------------------------------------------------------- test stage
# Built only when targeted: `docker build --target test .`
FROM build AS test
RUN npm run lint && npm test

# --------------------------------------------------------------- runtime stage
FROM nginx:1.29-alpine AS runtime

# The template is rendered at start-up with envsubst, so API_UPSTREAM can point
# at a different API per environment without rebuilding the image. The filter
# keeps envsubst away from nginx's own $uri and $host variables.
ENV API_UPSTREAM=http://api:8080 \
    NGINX_ENVSUBST_FILTER=^API_

COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=2s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/ || exit 1
