# GPON Telemetry Dashboard

Small telemetry stack for RouterOS containers and Realtek/RTL960x-style GPON
ONU sticks whose Boa web UI exposes optical status at `/status_pon.asp`.

The container polls the GPON stick over HTTP, serves a static dashboard on port
`3000`, and builds week/month/year rollups from RouterOS-managed raw logs.
RouterOS owns the raw 24-hour log and disk rotation.

Background and hardware context:
https://marfillaster.github.io/converge-gpon-sfp-stick-mikrotik/

## Dashboard

![GPON telemetry dashboard](https://raw.githubusercontent.com/marfillaster/gpon-telemetry/main/docs/dashboard-screenshot.png)

## Tags

- `alpine` - Alpine-based arm64 image
- `alpine-arm64` - explicit arm64 Alpine tag, recommended for RouterOS

## Quick Start

Use this image in RouterOS:

```text
docker.io/marfillaster/gpon-telemetry:alpine-arm64
```

The RouterOS install script is maintained in the GitHub repository:

```text
routeros/install.remote-image.rsc
```

Set these placeholders before importing it:

```routeros
:local remoteImage "docker.io/marfillaster/gpon-telemetry:alpine-arm64"
:local storageRoot "<storage-volume>"
:local containerIPv4 "<container-ipv4-cidr>"
:local gatewayIPv4 "<router-ipv4>"
:local stickHost "<gpon-stick-host>"
:local stickUser "admin"
:local stickPass "admin"
```

Then import the script on RouterOS and open the dashboard at the container IP
on port `3000`.

## Container Environment

| Variable | Default | Meaning |
|---|---:|---|
| `GPON_ADDR` | `:3000` | Dashboard listen address |
| `GPON_STATIC_ROOT` | `/opt/gpontelemetry/www` | Static dashboard files |
| `GPON_LOG_ROOT` | `/var/lib/gpontelemetry` | Mounted RouterOS log directory |
| `GPON_HOST` | `192.168.1.1` | GPON stick host or full URL |
| `GPON_USER` | `admin` | Web UI username |
| `GPON_PASS` | `admin` | Web UI password |

## Compatibility

Known-good target:

- ODI DFP-34X-2C2, stock `V1.0-220923` firmware

Likely-adjacent targets include other ODI/VSOL/RTL960x sticks using the same
Realtek SDK web pages.

Expected firmware behavior:

- Login at `/boaform/admin/formLogin`
- PON status at `/status_pon.asp`
- Logout at `/boaform/admin/formLogout`
- Status labels for temperature, voltage, Tx power, Rx power, bias current,
  and ONU state

## Links

- Blog post: https://marfillaster.github.io/converge-gpon-sfp-stick-mikrotik/
- GitHub: https://github.com/marfillaster/gpon-telemetry
- Release: https://github.com/marfillaster/gpon-telemetry/releases/tag/v2026.05.19
- License: MIT
