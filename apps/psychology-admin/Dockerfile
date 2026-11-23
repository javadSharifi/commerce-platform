# apps/psychology-admin/Dockerfile
FROM node:22-alpine AS builder

WORKDIR /app

RUN npm install -g pnpm@10.12.1 && pnpm config set registry https://registry.npmmirror.com

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/psychology-api-client ./packages/psychology-api-client
COPY packages/ui ./packages/ui
COPY apps/psychology-admin ./apps/psychology-admin

RUN pnpm install --frozen-lockfile
RUN pnpm --filter @psychology/api-client build
RUN pnpm --filter @psychology/ui build
RUN pnpm --filter psychology-admin build

FROM node:22-alpine AS runner

WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=4321

COPY --from=builder /app/package.json /app/pnpm-workspace.yaml ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/packages ./packages
COPY --from=builder /app/apps/psychology-admin ./apps/psychology-admin

WORKDIR /app/apps/psychology-admin

EXPOSE 4321

USER node

CMD ["node", "./dist/server/entry.mjs"]

