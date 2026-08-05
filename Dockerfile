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
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
WORKDIR /src

COPY . .

# 将前端构建产物放入后端可托管目录
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist

RUN go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
    && go build -o /out/server ./cmd/server

#################
# Runtime stage #
#################
FROM acr-openxlab-prod-registry-vpc.cn-shanghai.cr.aliyuncs.com/public/nginx:1.27-alpine
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data/uploads/icons

COPY --from=backend-builder /out/server /app/server
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html
COPY deploy/nginx/nginx.conf /etc/nginx/nginx.conf
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY docker/start.sh /start.sh

RUN chmod +x /start.sh

# 建议挂载真实配置到 /app/config.yaml
# ENV CONFIG_FILE=/app/config.yaml

EXPOSE 80

ENTRYPOINT ["/start.sh"]
