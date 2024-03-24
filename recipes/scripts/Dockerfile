FROM python:3.9

WORKDIR /app

COPY refresh_token.py .

RUN pip install --no-cache-dir kubernetes

CMD ["python", "./refresh_token.py"]
