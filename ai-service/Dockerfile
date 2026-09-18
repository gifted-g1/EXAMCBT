FROM python:3.12-slim

# OpenCV needs a couple of shared libs not present in the slim image.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

EXPOSE 9000
ENV AI_SERVICE_PORT=9000
ENV AI_SERVICE_HOST=0.0.0.0

CMD ["python", "-m", "app.main"]
