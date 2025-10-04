# syntax=docker/dockerfile:1.6

FROM node:22-bookworm AS builder
WORKDIR /workspace

# Install and build local SDK dependency
COPY packages/sdk-ts ./packages/sdk-ts
RUN --mount=type=cache,target=/root/.npm npm --prefix packages/sdk-ts ci
RUN --mount=type=cache,target=/root/.npm npm --prefix packages/sdk-ts run build

# Prime npm install cache with UI manifests and install deps
COPY apps/ui/package.json apps/ui/package-lock.json ./apps/ui/
WORKDIR /workspace/apps/ui
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy the rest of the UI source and build
COPY apps/ui/ .
ARG VITE_API_BASE=http://localhost:8080
ARG VITE_API_WS=ws://localhost:8080
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_API_WS=$VITE_API_WS
RUN npm run build

FROM nginx:1.27-alpine
COPY apps/ui/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/apps/ui/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
