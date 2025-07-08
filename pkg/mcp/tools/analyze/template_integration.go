package analyze

import (
	"fmt"
	"log/slog"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// TemplateIntegration handles Dockerfile template integration
type TemplateIntegration struct {
	logger *slog.Logger
}

// NewTemplateIntegration creates a new template integration handler
func NewTemplateIntegration(logger *slog.Logger) *TemplateIntegration {
	return &TemplateIntegration{
		logger: logger,
	}
}

// GetTemplateContent returns the content for a specific template
func (ti *TemplateIntegration) GetTemplateContent(templateName string) (string, error) {
	templates := map[string]string{
		"go":      ti.getGoTemplate(),
		"node":    ti.getNodeTemplate(),
		"python":  ti.getPythonTemplate(),
		"java":    ti.getJavaTemplate(),
		"rust":    ti.getRustTemplate(),
		"php":     ti.getPHPTemplate(),
		"ruby":    ti.getRubyTemplate(),
		"dotnet":  ti.getDotNetTemplate(),
		"generic": ti.getGenericTemplate(),
	}

	if content, ok := templates[templateName]; ok {
		return content, nil
	}

	return "", errors.NewError().Messagef("template not found: %s", templateName).WithLocation().Build()
}

func (ti *TemplateIntegration) ApplyTemplate(template string, vars map[string]string) string {
	result := template

	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// Template definitions

func (ti *TemplateIntegration) getGoTemplate() string {
	return `FROM {{BASE_IMAGE}}

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

EXPOSE 8080
CMD ["./main"]`
}

func (ti *TemplateIntegration) getNodeTemplate() string {
	return `FROM {{BASE_IMAGE}}

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

EXPOSE 3000
CMD ["node", "index.js"]`
}

func (ti *TemplateIntegration) getPythonTemplate() string {
	return `FROM {{BASE_IMAGE}}

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000
CMD ["python", "app.py"]`
}

func (ti *TemplateIntegration) getJavaTemplate() string {
	return `FROM {{BASE_IMAGE}} AS build

WORKDIR /app

COPY pom.xml .
RUN mvn dependency:go-offline

COPY src ./src
RUN mvn package

FROM openjdk:17-jre-slim
WORKDIR /app
COPY --from=build /app/target/*.jar app.jar

EXPOSE 8080
CMD ["java", "-jar", "app.jar"]`
}

func (ti *TemplateIntegration) getRustTemplate() string {
	return `FROM {{BASE_IMAGE}} AS builder

WORKDIR /app

COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release && rm -rf src

COPY src ./src
RUN cargo build --release

FROM debian:bullseye-slim
WORKDIR /app
COPY --from=builder /app/target/release/app /app/

EXPOSE 8080
CMD ["./app"]`
}

func (ti *TemplateIntegration) getPHPTemplate() string {
	return `FROM {{BASE_IMAGE}}

RUN docker-php-ext-install pdo pdo_mysql

WORKDIR /var/www/html

COPY composer.json composer.lock ./
RUN composer install --no-dev --optimize-autoloader

COPY . .
RUN chown -R www-data:www-data /var/www/html

EXPOSE 9000
CMD ["php-fpm"]`
}

func (ti *TemplateIntegration) getRubyTemplate() string {
	return `FROM {{BASE_IMAGE}}

RUN apt-get update -qq && apt-get install -y nodejs postgresql-client

WORKDIR /app

COPY Gemfile Gemfile.lock ./
RUN bundle install

COPY . .

EXPOSE 3000
CMD ["rails", "server", "-b", "0.0.0.0"]`
}

func (ti *TemplateIntegration) getDotNetTemplate() string {
	return `FROM {{BASE_IMAGE}} AS build

WORKDIR /app

COPY *.csproj ./
RUN dotnet restore

COPY . .
RUN dotnet publish -c Release -o out

FROM mcr.microsoft.com/dotnet/aspnet:7.0-alpine
WORKDIR /app
COPY --from=build /app/out .

EXPOSE 80
ENTRYPOINT ["dotnet", "App.dll"]`
}

func (ti *TemplateIntegration) getGenericTemplate() string {
	return `FROM {{BASE_IMAGE}}

WORKDIR /app

COPY . .

EXPOSE 8080
CMD ["./run.sh"]`
}
