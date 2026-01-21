# Test: block-deprecated-python (FAIL)
# Violation: Uses deprecated Python 3.6
FROM python:3.6

WORKDIR /app

COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

USER app
EXPOSE 8000

CMD ["python", "app.py"]
