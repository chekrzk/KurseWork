FROM node:24-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_DEMO_MODE=false
RUN VITE_DEMO_MODE=$VITE_DEMO_MODE npm run build
RUN mkdir -p node_modules/.vite-temp && chown node:node node_modules/.vite-temp
USER node
CMD ["npm","run","preview"]
