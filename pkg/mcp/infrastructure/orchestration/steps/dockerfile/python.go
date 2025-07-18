package dockerfile

// PythonTemplate provides the Dockerfile template for Python applications
const PythonTemplate = `FROM python:{{.LanguageVersion}}-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
{{if .Port -}}
EXPOSE {{.Port}}
{{else -}}
EXPOSE 5000
{{end -}}
{{if .IsDjango -}}
CMD ["python", "manage.py", "runserver", "0.0.0.0:8000"]
{{else if .IsFastAPI -}}
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
{{else -}}
CMD ["python", "app.py"]
{{end -}}
`
