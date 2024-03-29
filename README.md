# rffmpeg-autoscaler

Automagically scale number of rffmpeg workers in the cloud

## NOTICE!
**This is a rewrite of [Hcloud Rffmpeg](https://github.com/aleksasiriski/hcloud-rffmpeg) and currently it only supports Hetzner Cloud.**

## Kubernetes

On Kubernetes you can use [OpenEBS](https://github.com/openebs/dynamic-nfs-provisioner) to create RWX from RWO volume or [Longhorn](https://longhorn.io) RWX volumes (NFSv4) and mount said paths to Jellyfin host and workers (must be exactly the same mount points!).

Here's a [Helm chart repo with instuctions](https://github.com/aleksasiriski/jellyfin-kubernetes)

## Setup

1) Use this image: `ghcr.io/aleksasiriski/rffmpeg-autoscaler:latest`
1) Set the required environment variables
1) Share the Jellyfin config volume with this image (the only things that are really needed: Sqlite DB that rffmpeg uses if you haven't setup Postgres and rffmpeg's public ssh key, other than that it's convenient for **rffmpeg-autoscaler** to store log file inside Jellyfin's `/config/log` dir because you can read the log from Jellyfin's interface)
1) All of your workers need Jellyfin's `/config/transcodes` and `/config/data/subtitles` directories available and mounted at the same path as the Jellyfin host. Best solution is to use `NFSv4`.
1) Also, I recommend using Hetzner Storage Box for media share (cifs/samba) and setting `MEDIA_USERNAME` and `MEDIA_PASSWORD`, but if you aren't using it you will need to share media directory with the workers as well.

If you need a reference docker compose file with NFS server use [this one](https://github.com/aleksasiriski/rffmpeg-autoscaler/blob/main/docker-compose.example.yml).

## Recommended images

I made and tested these images to use with this script:

* [ghcr.io/aleksasiriski/jellyfin-rffmpeg](https://github.com/aleksasiriski/jellyfin-rffmpeg)
* [ghcr.io/aleksasiriski/rffmpeg-worker](https://github.com/aleksasiriski/rffmpeg-worker)

## Environment variables

| Name			| Default value		| Description		|
| :----------: | :--------------: | :--------------- | 
| RFFMPEG_AUTOSCALER_CONFIG | /config | Path to config dir |
| RFFMPEG_AUTOSCALER_LOG | /config/log/rffmpeg-autoscaler.log | Path to the log file |
| RFFMPEG_AUTOSCALER_VERBOSITY | 0 | 1 means DEBUG, 2 means TRACE |
| JELLYFIN_HOST | Must be explicitly set! | The IP address or hostname of Jellyfin's NFS share that workers use to access transcodes and subtitles directories |
| JELLYFIN_SSH_KEY | /config/rffmpeg/.ssh/id_ed25519.pub | Path to rffmpeg public ssh key generated on the Jellyfin host |
| JELLYFIN_JOBS | 2 | Number of jobs allowed per worker, the default of 2 tells the script to only create a new worker if there are 2 or more jobs on the previous one. |
| JELLYFIN_WEIGHT | 1 | Weight of the workers, higher numbers meaning they are more prefered for transcoding. Useful only if you have added custom workers or when using multiple scripts like this one |
| HETZNER_TOKEN | Must be explicitly set! | Hetzner Cloud API token |
| HETZNER_SERVER | cpx21 | The type of server from Hetzner that should be used for workers |
| HETZNER_IMAGE | docker-ce | The OS image used on workers, `docker-ce` is Ubuntu with Docker preinstalled |
| HETZNER_SSH_KEY | root@jellyfin | The name of the ssh key that will be saved on Hetzner and used for connecting to workers |
| HETZNER_NETWORK | rffmpeg-workers | The name of the network created for local communication between the workers and the Jellyfin host
| HETZNER_FIREWALL | rffmpeg-workers | The name of the firewall created for workers, recommended to block access to ssh over the internet
| HETZNER_PLACEMENT_GROUP | rffmpeg-workers | The name of the placement group created to spread the workers over the datacenter |
| HETZNER_LOCATION | nbg1 | The name of the location in which the workers should be created |
| HETZNER_CLOUD_INIT | [string](https://github.com/aleksasiriski/rffmpeg-autoscaler/blob/main/config.go#L68) | The string that setups the workers after creation, the default uses my docker compose and inserts needed env variables |
| DATABASE_TYPE | sqlite | Must be 'sqlite' or 'postgres` |
| DATABASE_PATH | /config/rffmpeg/db/rffmpeg.db | Path to the SQLite DB, ignored when type is postgres |
| DATABASE_HOST | localhost | Postgres database host |
| DATABASE_PORT | 5432 | Postgres database port |
| DATABASE_NAME | rffmpeg | Postgres database name |
| DATABASE_USERNAME | postgres | Postgres database username |
| DATABASE_PASSWORD | "" | Postgres database password |
| MEDIA_USERNAME | "" | Username for the Storage Box media share |
| MEDIA_PASSWORD | "" | Password for the Storage Box media share |
