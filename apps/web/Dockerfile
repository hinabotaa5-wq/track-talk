FROM node:26-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# NEXT_PUBLIC_* はビルド時にブラウザ側へ埋め込まれるので、ローカル確認用の値をここで渡す。
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ARG NEXT_PUBLIC_SUPABASE_URL=http://localhost:9998
ARG NEXT_PUBLIC_SUPABASE_ANON_KEY=local-anon-key
ARG NEXT_PUBLIC_COACHING_ENABLED=1
ENV NEXT_PUBLIC_COACHING_ENABLED=$NEXT_PUBLIC_COACHING_ENABLED
ARG NEXT_PUBLIC_SITE_URL=http://localhost:3000
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL \
    NEXT_PUBLIC_SUPABASE_URL=$NEXT_PUBLIC_SUPABASE_URL \
    NEXT_PUBLIC_SUPABASE_ANON_KEY=$NEXT_PUBLIC_SUPABASE_ANON_KEY \
    NEXT_PUBLIC_SITE_URL=$NEXT_PUBLIC_SITE_URL \
    NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:26-alpine
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1
COPY --from=build /app/package.json /app/package-lock.json ./
COPY --from=build /app/node_modules ./node_modules
# Next.js の実行時キャッシュを一般ユーザーで書き込めるようにする。
COPY --from=build --chown=node:node /app/.next ./.next
COPY --from=build /app/public ./public
EXPOSE 3000
USER node
CMD ["npm", "run", "start"]
