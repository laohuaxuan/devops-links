########################
# Frontend build stage #
########################
FROM acr-openxlab-prod-registry-vpc.cn-shanghai.cr.aliyuncs.com/public/node:18.20 AS frontend-builder
WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

#######################
# Backend build stage #
#######################
FROM acr-openxlab-prod-registry-vpc.cn-shanghai.cr.aliyuncs.com/public/golang:1.25.6 AS backend-builder
WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY . .

# 将前端构建产物放入后端可托管目录
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
    && go build -ldflags="-s -w" -o /out/server ./cmd/server \
    && test -f /out/server

#################
# Runtime stage #
#################
FROM acr-openxlab-prod-registry-vpc.cn-shanghai.cr.aliyuncs.com/public/nginx:1.27-alpine
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data/uploads/icons

COPY --from=backend-builder /out/server /usr/local/bin/devops-links-server
RUN chmod +x /usr/local/bin/devops-links-server \
    && ln -sf /usr/local/bin/devops-links-server /app/server \
    && test -x /usr/local/bin/devops-links-server
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html
COPY deploy/nginx/nginx.conf /etc/nginx/nginx.conf
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY docker/start.sh /start.sh

RUN chmod +x /start.sh

# 建议挂载真实配置到 /app/config.yaml
# ENV CONFIG_FILE=/app/config.yaml

EXPOSE 80

ENTRYPOINT ["/start.sh"]
