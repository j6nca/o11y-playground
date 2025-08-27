# monitoring-workshop-2509

This project contains files to spin up a standalone environment, to get folks familiar with application monitoring.

## Setup

```
# Start services in detached mode
docker-compose up -d

# Ensure everything we just spun up is running
docker-compose ps
```

## Hitting app /metrics endpoint

```
curl localhost:$APP_PORT/metrics
```
