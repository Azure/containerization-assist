/**
 * Python Flask Basic Repository Fixture
 * Simple Flask application for testing
 */

export const pythonFlaskBasicRepository = {
  'requirements.txt': `Flask==2.3.3
python-dotenv==1.0.0
gunicorn==21.2.0`,
  'app.py': `from flask import Flask, jsonify
import os
from datetime import datetime

app = Flask(__name__)

@/app.route('/')
def hello():
    return jsonify({
        'message': 'Hello World!',
        'timestamp': datetime.now().isoformat()
    })

@/app.route('/health')
def health():
    return jsonify({
        'status': 'healthy',
        'version': '1.0.0'
    })

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(host='0.0.0.0', port=port, debug=False)`,
  'runtime.txt': 'python-3.11.5',
  Procfile: 'web: gunicorn app:app',
  '.env.example': `PORT=5000
FLASK_ENV=production`,
  'README.md': `# Python Flask Basic

A simple Flask application for testing containerization.

## Running the application

\`\`\`bash
pip install -r requirements.txt
python app.py
\`\`\`

The server will start on port 5000.`,
  '.gitignore': `__pycache__/
*.py[cod]
*$py.class
*.so
.Python
venv/
env/
.env
.venv/
pip-log.txt
pip-delete-this-directory.txt
.coverage
htmlcov/`,
};

export const expectedPythonFlaskAnalysis = {
  projectType: 'python',
  packageManager: 'pip',
  buildTool: 'pip',
  dependencies: ['Flask', 'python-dotenv', 'gunicorn'],
  devDependencies: [],
  requirements: 'requirements.txt',
  ports: [5000],
  pythonVersion: '3.11.5',
  hasDockerfile: false,
  hasTests: false,
  entrypoint: 'app.py',
};

export const expectedPythonFlaskDockerfile = `FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 5000

RUN adduser --disabled-password --gecos '' appuser && chown -R appuser:appuser /app
USER appuser

CMD ["gunicorn", "--bind", "0.0.0.0:5000", "app:app"]`;

export {};
