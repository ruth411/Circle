# Snyk workflow

Run the phase checkpoint loop with:

```bash
make phase-check PHASE=phase-0
make phase-check PHASE=phase-0 ITER=fix-1
```

What it does:

- runs `snyk test`
- runs `snyk code test`
- runs `snyk monitor`
- saves logs and JSON outputs under `reports/`

Exit behavior:

- exit code `0` means scans ran and no issues were found
- exit code `1` from Snyk means issues were found
- the wrapper exits non-zero only when a scan/monitor fails technically, not
  just because findings exist
