# Loki

Log aggregation and search.

**Status:** implemented (Phase 2, `local` preset overlay)

## Purpose

Log aggregation and search.

## Provided

- Single-binary Loki with filesystem storage and 72h retention.
- Promtail ships container logs discovered through the Docker socket, limited to the `forgelab-local` compose project and labelled with `service`, `container`, and `level`.

## Dependencies

- Compute (Docker)
- Promtail (log shipper, part of this component)

## Configuration

`environments/local/observability/loki/loki.yaml` and `environments/local/observability/promtail/promtail.yaml`.
