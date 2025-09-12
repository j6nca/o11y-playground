# monitoring-workshop-2509

> [!warning]
> This project is still under frequent development and iteration, it is recommended before starting to run `git pull` to ensure code and configurations are up-to-date!

This project contains files to spin up a standalone environment, to get folks familiar with application monitoring. It extends the rideshare example from [grafana/pyroscope](https://github.com/grafana/pyroscope/tree/main/examples/tracing/golang-push).

## Setup

### Requirements

You will need `docker`, `docker compose` which can be installed via the following:

```
$ brew install colima docker-compose docker docker-buildx hey
$ export DOCKER_HOST="unix://$HOME/.colima/docker.sock"
$ colima start
```

Note: With homebrew installed docker-compose and docker-buildx, you may need to update your docker config to include the following to properly reference the plugins.

```
# In your ~/.docker/config.json:
{
  ...
  "cliPluginsExtraDirs": [
      "/opt/homebrew/lib/docker/cli-plugins"
  ]
}
```

### Setup

Please run through this setup prior to the workshop & travels (ideally before you get stuck on shared hotel wifi 😅).

```
# Start services in detached mode
# We are using '--build' here to let us make changes to the go code on the fly
$ docker-compose up -d --wait --build
[+] Running 18/18
 ✔ Container workshop-ap-south-1           Built
 ✔ Container workshop-eu-north-1           Built
 ✔ Container workshop-us-east-1            Built
 ✔ Container workshop-load-generator-1     Built
 ✔ Network monitoring-workshop             Created
 ✔ Container gatus                         Healthy
 ✔ Container pyroscope                     Healthy
 ✔ Container alertmanager                  Healthy
 ✔ Container grafana                       Healthy
 ✔ Container example-app                   Healthy
 ✔ Container loki                          Healthy
 ✔ Container tempo                         Healthy
 ✔ Container example-app-2                 Healthy
 ✔ Container alloy                         Healthy
 ✔ Container vmstorage                     Healthy
 ✔ Container vmselect                      Healthy
 ✔ Container vminsert                      Healthy
 ✔ Container vmalert                       Healthy
```

If everything shows as `Created`, `Started` or `Healthy`, you are good to go! We can clean this up for now:

```
# Start services in detached mode
$ docker-compose down
[+] Running 16/16
 ✔ Container gatus                         Removed
 ✔ Container pyroscope                     Removed
 ✔ Container alertmanager                  Removed
 ✔ Container grafana                       Removed
 ✔ Container loki                          Removed
 ✔ Container tempo                         Removed
 ✔ Container workshop-ap-south-1           Removed
 ✔ Container workshop-eu-north-1           Removed
 ✔ Container workshop-us-east-1            Removed
 ✔ Container workshop-load-generator-1     Removed
 ✔ Container alloy                         Removed
 ✔ Container vmstorage                     Removed
 ✔ Container vmselect                      Removed
 ✔ Container vminsert                      Removed
 ✔ Container vmalert                       Removed
 ✔ Network monitoring-workshop             Removed
```

### Workshop Setup

If internet permits, please run `git pull` prior to the workshop just to ensure files are up-to-date.

```
docker-compose up -d --wait --build
```

## Workshop

During the workshop, you may want to make code changes and re-deploy services, to do so, please modify the code in respective directories and re-deploy them via:

```
docker-compose up -d --wait --build
```

### Accessing the services

The provisioned [Monitoring Workshop > Monitoring Workshop](http://localhost:3000/d/rideshare/rideshare-example?orgId=1&from=now-5m&to=now&timezone=browser&refresh=10s) dashboard also includes links to the following for ease of reference.

#### Working Examples (You will mostly interact with these apps)

- [grafana](http://localhost:3000)
- [vmalert](http://localhost:8880)
- [alertmanager](http://localhost:9093)
- [alloy](http://localhost:12345)
- [gatus](http://localhost:8888)
- [rideshare-us-east](http://localhost:5050)
- [rideshare-eu-north](http://localhost:5051)

#### Observability Stack

- [loki](http://localhost:3100)
- [pyroscope](http://localhost:4040)
- [tempo](http://localhost:3200)
- [vmselect](http://localhost:8481)
- [vmstorage](http://localhost:8401)
- [vminsert](http://localhost:8480)

### Metrics

Restart one instance of rideshare app

```
docker-compose restart us-east-1
```

Request throughput query

```
sum(rate(go_app_http_requests_total{service_name=~"workshop.*"}[5m]))
```

Hit error endpoint with `hey`

```
hey -n 50 http://localhost:5050/error
```

Request errors query

```
sum(rate(go_app_http_requests_total{service_name=~"workshop.*", status_code=~"5.."}[5m]))/
sum(rate(go_app_http_requests_total{service_name=~"workshop.*"}[5m]))
```

Request latency query

```
sum(
rate(go_app_http_request_duration_seconds_sum{service_name=~"workshop.*", path!="/favicon.ico"}[5m])
/
rate(go_app_http_request_duration_seconds_count{service_name=~"workshop.*", path!="/favicon.ico"}[5m])
) by (path)
```

### Logging

### Traces and Profiling

Rebuild rideshare services after fixing code

```
docker-compose up -d --wait --build
```

### Alerts

Hit error endpoint with `hey`

```
hey -n 200 http://localhost:5050/error
```

Restart vmalert

```
docker-compose restart vmalert
```

## Cleanup

After the workshop, please spin down services and you can then remove relevant files locally:

```
docker-compose down
```

> [!warning]
> After the workshops afternoon you may want to prune locally pulled images. Don't do this until the end of the day or you may have to re-pull images during the afternoon!