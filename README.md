# monitoring-workshop-2509

> [!warning]
> This project is still under frequent development and iteration, it is recommended before starting to run `git pull` to ensure code and configurations are up-to-date!

This project contains files to spin up a standalone environment, to get folks familiar with application monitoring.

## Requirements

You will need `docker`, `docker compose` which can be installed via the following:

```
$ brew install colima docker-compose docker docker-buildx
$ export DOCKER_HOST="unix://$HOME/.colima/docker.sock"
$ colima start
```

### Setup

Please run through this setup prior to the workshop & travels (ideally before you get stuck on shared hotel wifi 😅).

```
# Start services in detached mode
# We are using '--build' here to let us make changes to the go code on the fly
$ docker-compose up -d --wait --build
[+] Running 16/16
 ✔ example-app                  Built
 ✔ example-app-2                Built
 ✔ Network monitoring-workshop  Created
 ✔ Container gatus              Healthy
 ✔ Container pyroscope          Healthy
 ✔ Container alertmanager       Healthy
 ✔ Container grafana            Healthy
 ✔ Container example-app        Healthy
 ✔ Container loki               Healthy
 ✔ Container tempo              Healthy
 ✔ Container example-app-2      Healthy
 ✔ Container alloy              Healthy
 ✔ Container vmstorage          Healthy
 ✔ Container vmselect           Healthy
 ✔ Container vminsert           Healthy
 ✔ Container vmalert            Healthy
```

If everything shows as `Created`, `Started` or `Healthy`, you are good to go! We can clean this up for now:

```
# Start services in detached mode
$ docker-compose down
[+] Running 14/14
 ✔ Container gatus              Removed
 ✔ Container pyroscope          Removed
 ✔ Container alertmanager       Removed
 ✔ Container grafana            Removed
 ✔ Container example-app        Removed
 ✔ Container loki               Removed
 ✔ Container tempo              Removed
 ✔ Container example-app-2      Removed
 ✔ Container alloy              Removed
 ✔ Container vmstorage          Removed
 ✔ Container vmselect           Removed
 ✔ Container vminsert           Removed
 ✔ Container vmalert            Removed
 ✔ Network monitoring-workshop  Removed
```

### Workshop Setup

If internet permits, please run `git pull` prior to the workshop just to ensure files are up-to-date.

```
docker-compose up -d --wait
```
