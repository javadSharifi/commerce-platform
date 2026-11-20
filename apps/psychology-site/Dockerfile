# apps/psychology-site/Dockerfile
FROM node:22-alpine AS builder

WORKDIR /app

RUN npm install -g pnpm@10.5.2

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/psychology-api-client ./packages/psychology-api-client
COPY apps/psychology-site ./apps/psychology-site

RUN pnpm install --frozen-lockfile
RUN pnpm --filter @psychology/api-client build
RUN pnpm --filter psychology-site build

FROM node:22-alpine AS runner

WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=4322

COPY --from=builder /app/apps/psychology-site/dist ./dist
COPY --from=builder /app/apps/psychology-site/node_modules ./node_modules
COPY --from=builder /app/apps/psychology-site/package.json ./package.json

EXPOSE 4322

USER node

CMD ["node", "./dist/server/entry.mjs"]
