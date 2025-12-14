# Zenit

<!-- markdownlint-disable-next-line MD033 -->
<img src="assets/img/zenit.svg" alt="Zenit" align="right" width="200">

Lightweight telemetry aggregation server.
Named after the "Radio Zenit" broadcasting station in Chernarus (DayZ),
this service was originally designed to track usage of server-side
DayZ modifications.

However, it is agnostic and can be used for:

* **Generic Applications**:
  Tracking version distribution and uptime via simple heartbeats.
* **Game Servers**:
  Any game server compatible with the Valve Source Query Protocol (`A2S_INFO`).

<!-- markdownlint-disable-next-line MD033 -->
<br clear="right"/>

## Install

Download binary from
[releases page](https://github.com/WoozyMasta/zenit/releases)
or build it, just run `make build` or use container image.

## Container

Container images are available at:

* `ghcr.io/woozymasta/zenit:latest`
* `docker.io/woozymasta/zenit:latest`

```bash
mkdir -p data
docker run -d --name zenit \
  -p 8080:8080 \
  -v "$PWD/data:/data" \
  ghcr.io/woozymasta/zenit:latest --auth-token=admin
```

## Configuration

The application is configured via command-line flags or environment variables.  
For a complete list of available options, run:

```bash
./zenit --help
```

> [!NOTE]  
> An authentication token
> (`--auth-token` or `ZENIT_AUTH_TOKEN`) is required to start the server.

## Endpoints

### Public

* `POST /api/telemetry` - Ingests telemetry data. Expects JSON.

```json
{
  "application": "MetricZ",
  "version": "1.1.0",
  "type": "steam",
  "port": 27016
}
```

### Administrative

Protected via HTTP Basic Auth or Bearer token.

* `GET /dashboard` - Web interface.
* `GET /api/stats` - Returns all nodes as JSON.
* `GET /api/node` - Node details.
* `GET /api/a2s` - Proxy A2S query to a remote server.
* `DELETE /api/node` - Remove node.

## Maintenance

The binary supports standalone maintenance modes to clean up the database.
These commands exit after completion.

* `--prune-empty` - Delete records with no A2S data.
* `--check-inactive` - Re-check servers not seen recently; update or delete.
* `--check-all` - Re-check all servers.

## Storage

State is maintained in a SQLite database (`zenit.db`)
and a MaxMind GeoIP database (`zenit.mmdb`).
Both are stored in the working directory or the path specified via flags.
