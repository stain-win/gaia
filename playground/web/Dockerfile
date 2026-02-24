FROM node:20-alpine AS base

# Install dependencies only when needed
FROM base AS deps
# Check https://github.com/nodejs/docker-node/tree/b4117f9333da4138b03a546ec926ef50a31506c3#nodealpine to understand why libc6-compat might be needed.
RUN apk add --no-cache libc6-compat
COPY playground/web/package.json playground/web/package-lock.json* ./

# First, build and pack the local JS client so "dist/" is present
WORKDIR /workspace/libs/js
COPY libs/js ./
RUN npm install
RUN npm run build
RUN npm pack

# Install dependencies for web, pointing to the tarball
WORKDIR /workspace/playground/web
COPY playground/web ./
# Manually install the packed client and then install the rest
RUN npm install ../../libs/js/stain-win-gaia-client-0.1.0.tgz
RUN npm ci

# Rebuild the source code only when needed
FROM base AS builder
WORKDIR /workspace/playground/web
COPY --from=deps /workspace/playground/web ./

ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# Production image, copy all the files and run next
FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Set the correct permission for prerender cache
RUN mkdir .next
RUN chown nextjs:nodejs .next

# Automatically leverage output traces to reduce image size
# https://nextjs.org/docs/advanced-features/output-file-tracing
COPY --from=builder --chown=nextjs:nodejs /workspace/playground/web/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /workspace/playground/web/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /workspace/playground/web/public ./public
# Only copy the proto directory from the local client, since Next.js standalone trace misses non-JS files
COPY --from=deps /workspace/playground/web/node_modules/@stain-win/gaia-client/proto ./node_modules/@stain-win/gaia-client/proto

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
