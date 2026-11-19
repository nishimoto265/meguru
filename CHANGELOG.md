# Changelog

## v0.1.0 — 2026-11-19

First public release.

### Included

- Loop definitions in a single YAML file: purpose, metrics (current /
  target / measured_by), stage design (split unit, `parallel_max` or
  `parallel_note`, reason), improvement history, and next candidates
- `validate`: structural checks — purpose present, every metric has a
  target and a measurement source, every stage has a split unit, a
  reason, and a parallelism design value, and every improvement's
  `aimed_at` points at a defined metric
- `report`: Markdown / JSON digest of loops — metric current → target,
  current stage and cycle count, recent improvements (up to 3), and
  next candidates
- `check`: drift and neglect detection — stale metrics (default 30
  days), improvements without a recorded result, and (with `--actuals`)
  actual parallelism exceeding the designed `parallel_max`; exit code 1
  when findings exist
- Actuals ingestion: read-only JSONL with lenient field-name matching
  (stage / started_at / finished_at / status and their aliases),
  aggregated per stage into runs, failure rate, average duration, and
  maximum concurrency; unreadable lines are warned about and skipped

### Known limitations

- Single machine, single operator; definitions and actuals are local
  files only, no server mode or sync
- Actuals ingestion is JSONL only; no CSV or OpenTelemetry input yet
- No custom rule language; detection is the three fixed checks above
- Validated by dogfooding against two synthetic example loops (no false
  positives). Ingestion of actuals exported from michaku is future work.
