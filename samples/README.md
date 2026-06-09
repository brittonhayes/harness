# Sample evidence

Offline log events for driving a hunt without a live data lake. Point vala's
file-backed `scanner` source at this directory and the `scanner_load_context` /
`scanner_execute_query` tools answer from these files:

```sh
SCANNER_SAMPLES_DIR=samples vala
```

Each file is one **index** (named after the file, minus extension). Events are
either a JSON array of objects or newline-delimited JSON (`.json`, `.jsonl`,
`.ndjson`). `cloudtrail.jsonl` carries a small AWS CloudTrail scenario — a
GuardDuty `DeleteDetector` and CloudTrail `StopLogging` by an IAM user, an
unauthenticated root `ConsoleLogin`, and benign activity to hunt against.

Drop in your own files to hunt different data; no schema is imposed.
