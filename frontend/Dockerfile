FROM node:18-alpine AS builder

WORKDIR /app
COPY package*.json ./
# Require npm install to succeed so we don't cache broken layers
RUN npm install
COPY . .
RUN npm run build

# Serve with nginx
FROM nginx:alpine
# If build fails locally due to no node modules, we just fall back, 
# but assuming node is available in the container:
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
