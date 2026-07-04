# platform-go

Reusable operations platform foundation extracted from the `zshenmez` platform work.

## Current Slice

This repository currently contains the first framework skeleton:

- platform kernel primitives;
- capability registry and dependency resolution;
- default core governance manifests;
- Gin HTTP runtime with health and capability introspection;
- minimal React admin shell.

## API

```bash
rtk proxy sh -lc 'PLATFORM_HTTP_ADDR=127.0.0.1:19200 go run ./cmd/platform-api & pid=$!; trap "kill $pid 2>/dev/null || true" EXIT; for i in $(seq 1 20); do curl -fsS http://127.0.0.1:19200/api/health && exit 0; sleep 0.5; done; exit 1'
```

Default API address:

```text
http://127.0.0.1:9200/api
```

Useful endpoints:

```text
GET /api/health
GET /api/capabilities
```

## Admin

```bash
rtk npm --prefix admin install
rtk npm --prefix admin run dev
```

Default admin address:

```text
http://127.0.0.1:9202
```

## Verification

```bash
rtk go test ./...
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
rtk git diff --check
```
