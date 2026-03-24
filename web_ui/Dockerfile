FROM node:22-slim AS build

COPY . /app

WORKDIR /app

RUN npm ci
RUN npm run build

FROM node:22-slim

COPY  --from=build /app/.output /app/.output
COPY --from=build /app/blocks /app/blocks
COPY --from=build /app/block-index.json /app/block-index.json

WORKDIR /app
EXPOSE 3000

ENTRYPOINT [ "node", ".output/server/index.mjs" ]



