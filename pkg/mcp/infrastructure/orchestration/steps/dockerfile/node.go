package dockerfile

// NodeTemplate provides the Dockerfile template for Node.js applications (JavaScript/TypeScript)
const NodeTemplate = `FROM node:{{.LanguageVersion}}-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
{{if .HasNextFramework -}}
RUN npm run build
{{end -}}
{{if .Port -}}
EXPOSE {{.Port}}
{{else -}}
EXPOSE 3000
{{end -}}
CMD ["npm", "start"]
`
