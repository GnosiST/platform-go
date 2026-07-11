FROM node:24-alpine AS admin-builder
WORKDIR /src/admin
COPY admin/package*.json ./
RUN npm ci
COPY admin/ ./
RUN npm run build

FROM golang:1.26-alpine AS api-builder
WORKDIR /src
RUN apk add --no-cache build-base ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY resources/generated/openapi.admin.json ./resources/generated/openapi.admin.json
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/platform-api ./cmd/platform-api
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/platform-admin ./cmd/platform-admin

FROM alpine:3.22 AS api
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S platform \
  && adduser -S -G platform platform \
  && mkdir -p /app/.platform/uploads /app/resources/generated \
  && chown -R platform:platform /app
COPY --from=api-builder /out/platform-api /usr/local/bin/platform-api
COPY --from=api-builder /out/platform-admin /app/platform-admin
COPY --from=api-builder /src/resources/generated/openapi.admin.json /app/resources/generated/openapi.admin.json
USER platform
EXPOSE 9200
ENV PLATFORM_HTTP_ADDR=0.0.0.0:9200 \
  PLATFORM_OPENAPI_FILE=resources/generated/openapi.admin.json
ENTRYPOINT ["platform-api"]

FROM nginx:1.29-alpine AS admin-static
COPY --from=admin-builder /src/admin/dist /usr/share/nginx/html
COPY deploy/nginx/platform.conf /etc/nginx/conf.d/default.conf
