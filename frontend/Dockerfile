# ==============================================================================
# Agentrix Platform - Frontend SPA Multi-Stage Dockerfile
# Builds the React + TypeScript app with Vite and serves via high-performance Nginx.
# ==============================================================================

# --- Stage 1: Build React SPA ---
FROM node:20-alpine AS builder
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/tsconfig.json frontend/vite.config.ts frontend/tailwind.config.js frontend/postcss.config.js frontend/index.html ./
COPY frontend/src ./src

RUN npm run build

# --- Stage 2: Serve via Nginx ---
FROM nginx:alpine AS runner

COPY --from=builder /app/dist /usr/share/nginx/html
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
