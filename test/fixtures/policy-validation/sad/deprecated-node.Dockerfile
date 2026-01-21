# Test: block-deprecated-node (FAIL)
# Violation: Uses deprecated Node.js 16
FROM node:16

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

USER node
EXPOSE 3000

CMD ["node", "server.js"]
