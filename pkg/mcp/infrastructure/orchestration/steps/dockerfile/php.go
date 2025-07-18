package dockerfile

// PHPTemplate provides the Dockerfile template for PHP applications
const PHPTemplate = `FROM php:{{.LanguageVersion}}-apache
WORKDIR /var/www/html
COPY . .
RUN chown -R www-data:www-data /var/www/html
{{if .Port -}}
EXPOSE {{.Port}}
{{else -}}
EXPOSE 80
{{end -}}
CMD ["apache2-foreground"]
`
