FROM node:20-alpine AS frontend-build
WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm config set registry https://registry.npmjs.org/ && npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-build
WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/emosup-server ./cmd/server

FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -h /app emosup \
    && mkdir -p /app/backend/data /app/frontend /app/defaults \
    && chown -R emosup:emosup /app

COPY --from=backend-build /out/emosup-server /app/backend/emosup-server
COPY --from=frontend-build /src/frontend/dist/ /app/frontend/
COPY backend/data/config.example.json /app/defaults/config.example.json

USER emosup
WORKDIR /app/backend

EXPOSE 8080

ENTRYPOINT ["./emosup-server"]
