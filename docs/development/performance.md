# Performance notes

Drovik's hot paths are bounded parsers and request/SSH lifecycle work, not long-running data
processing. Benchmarks live beside the parsers and run with:

```bash
make backend-bench
```

Reference run on an Apple M3 Pro (Go 1.27, arm64, 2026-11-08):

| Benchmark | Time | Allocations |
|---|---:|---:|
| `BenchmarkParseSample` (default 2-core host fixture) | ~5.2 µs | 15.6 KB / 89 |
| `BenchmarkParseProcessLine` | ~170 ns | 184 B / 2 |
| `BenchmarkParseJournalEntry` (JSON) | ~0.99 µs | 696 B / 16 |
| `BenchmarkParseUnits` (2 units + file state) | ~2.4 µs | 17.5 KB / 14 |

These are parser costs only. Network RTT to the managed host dominates real requests. The practical
load controls are:

- one short-lived SSH connection per operation;
- monitoring cached for `DROVIK_MONITOR_REFRESH_INTERVAL` and coalesced per host;
- process/service/journal output and entry counts bounded by configuration;
- SSH dial, handshake, command and HTTP timeouts;
- terminal input rate limits and bounded read sizes;
- upload/editor bounds and recursive-delete guards.

## Load-test guidance

For an operator deployment, test with representative RTT and real hosts rather than only local
fixtures:

1. Run 10–50 concurrent monitoring requests for one host and confirm one collection per refresh
   window (`cached: true` for the rest).
2. Open the maximum allowed terminals and confirm extra sessions receive a resource-limit error.
3. List a directory/service set at the configured caps and confirm truncation is reported.
4. Measure journal retrieval at 50/200/500/1000 lines on a busy host.
5. Confirm SQLite remains on local storage and WAL files are included in backups.

There is no committed load-test harness; results depend heavily on the remote host. Record the
host, RTT, caps and Drovik version with any performance report.
