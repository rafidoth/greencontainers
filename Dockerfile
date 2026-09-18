FROM ubuntu:latest
RUN apt-get update && apt-get install -y stress-ng
CMD ["stress-ng", "--cpu", "1", "--timeout", "60s"]
