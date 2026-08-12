ARG IMAGE_REGISTRY=dockerproxy.net
FROM ${IMAGE_REGISTRY}/library/nginx:1.28-alpine

RUN apk add --no-cache curl
COPY deploy/nginx/15-https-port.envsh /docker-entrypoint.d/15-https-port.envsh
RUN chmod 0755 /docker-entrypoint.d/15-https-port.envsh
