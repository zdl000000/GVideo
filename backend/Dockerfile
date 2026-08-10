ARG IMAGE_REGISTRY=docker.io
FROM ${IMAGE_REGISTRY}/library/golang:1.26-bookworm AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gvideo ./cmd/server

FROM ${IMAGE_REGISTRY}/library/debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home app
WORKDIR /app
COPY --from=build /out/gvideo /usr/local/bin/gvideo
RUN mkdir -p /app/data /app/media && chown -R app:app /app
USER app
EXPOSE 8080
CMD ["gvideo"]
