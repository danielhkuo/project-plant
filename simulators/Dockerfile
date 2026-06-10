# syntax=docker/dockerfile:1
# Device-fleet simulator. Build context is the simulators/ directory:
#   docker build -t plant-simulators simulators/
# Run a fleet against a stack:
#   docker run --rm plant-simulators --count 10 --profile tropical \
#     --api-url http://host.docker.internal:8080 --api-key dev-key-001

FROM python:3.12-slim
WORKDIR /app

COPY pyproject.toml ./
COPY src/ src/
RUN pip install --no-cache-dir .

ENTRYPOINT ["python", "-m", "simulators"]
CMD ["--help"]
