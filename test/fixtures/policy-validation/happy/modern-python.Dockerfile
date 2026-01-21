# Test: block-deprecated-python (PASS)
# Uses modern Python version (3.10+)
FROM mcr.microsoft.com/azurelinux/base/python:3.11

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

USER app
EXPOSE 8000

CMD ["python", "app.py"]
