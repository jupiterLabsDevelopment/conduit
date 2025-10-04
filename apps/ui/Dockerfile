# syntax=docker/dockerfile:1.6

FROM cgr.dev/chainguard/node:latest AS deps
WORKDIR /app
COPY apps/ui/package.json apps/ui/package-lock.json ./
RUN npm ci

FROM cgr.dev/chainguard/node:latest AS build
WORKDIR /app
ARG VITE_API_BASE=http://localhost:8080
ARG VITE_API_WS=ws://localhost:8080
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_API_WS=$VITE_API_WS
COPY --from=deps /app/node_modules ./node_modules
COPY apps/ui/ .
RUN npm run build

FROM cgr.dev/chainguard/nginx:latest
ENV LISTEN_PORT=4173
COPY apps/ui/docker/nginx.conf /etc/nginx/http.d/default.conf
COPY --from=build /app/dist /var/lib/nginx/html
EXPOSE 4173
