# Lineth quickstart scripts

Boot and inspection commands stay in this directory so the normal flow remains short:

- `start.sh`, `check-ports.sh`, `watch.sh`, `status.sh`, `links.sh`, `export-output.sh`
- `check-quickstart-static.sh`

Other scripts are split by purpose:

- `traffic-generation/` — optional local L2 activity helpers.
- `smoke-test/` — optional bridge acceptance checks.
- `phases/` — one-shot Docker Compose boot phases.
- `services/` — renderers for genesis and service runtime config.
- `internal/` — TypeScript helpers and deploy support files used by containers.
- `init/` — long-lived service entrypoints only.
- `lib/` — shared shell helpers.
