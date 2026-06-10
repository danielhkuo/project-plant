# syntax=docker/dockerfile:1

# ---- Builder ----
FROM node:22-alpine AS builder
WORKDIR /app

# Rewrite target for /api/* — baked at build time by next.config.ts
# (compose passes http://dashboard:8081; default suits local docker run).
ARG DASHBOARD_URL=http://localhost:8081
ENV DASHBOARD_URL=$DASHBOARD_URL

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

# ---- Runner ----
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000

CMD ["node", "server.js"]
