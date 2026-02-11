# Test: block-deprecated-node (PASS)
# Uses modern Node.js version (18+)
FROM mcr.microsoft.com/cbl-mariner/base/nodejs:20

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER node
EXPOSE 3000

CMD ["node", "server.js"]
