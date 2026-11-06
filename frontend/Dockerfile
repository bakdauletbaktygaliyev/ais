FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json ./
RUN npm install --legacy-peer-deps

COPY . .
RUN npm run build:prod

FROM nginx:alpine
RUN apk add --no-cache gettext
COPY --from=builder /app/dist/ais-frontend/browser /usr/share/nginx/html
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
CMD ["/bin/sh", "-c", "envsubst '${BACKEND_URL}${AI_SERVICE_URL}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"]
