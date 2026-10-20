# Observability Lab

A hands-on lab for understanding structured application logging, log
collection, centralized storage, search, and failure recovery.

## Current architecture

``` text
Go payment app → app.log → Fluent Bit → Elasticsearch → Kibana
```

-   **Go app:** writes structured JSON logs using `slog`.
-   **app.log:** source log file at the project root.
-   **Fluent Bit:** tails the file, parses JSON, and forwards records.
-   **Elasticsearch:** indexes logs.
-   **Kibana:** searches and displays indexed logs.

## Project layout

``` text
observability-lab/
├── app/
│   └── main.go
├── app.log
├── fluent-bit/
│   ├── fluent-bit.conf
│   └── parsers.conf
└── docker-compose.yml
```

Fluent Bit mounts the individual `./app.log` file at
`/var/log/app/app.log` inside its container. We intentionally avoid
mounting the entire application directory.

## Local endpoints

  Component       Address
  --------------- -----------------------
  Elasticsearch   http://localhost:9200
  Kibana          http://localhost:5601
  Log file        `./app.log`

The lab has used Elasticsearch and Kibana `8.17.0`, and Fluent Bit
`3.2.x` (observed version `3.2.10`). Elasticsearch security is disabled
for this local learning setup only; do not copy that setting blindly
into production.

## Example log record

``` json
{
  "time": "2026-10-20T12:00:00.000+05:30",
  "level": "INFO",
  "msg": "payment processing started",
  "service": "payment-service",
  "payment_id": "pay-123",
  "correlation_id": "req-12345"
}
```

Structured fields make it possible to filter by severity, service,
payment, or correlation ID.

## Fluent Bit configuration concepts

-   `INPUT` uses the `tail` plugin to read `/var/log/app/app.log`.
-   `Parser app_json` parses each JSON line.
-   `Read_from_Head true` reads existing file contents from the
    beginning when Tail discovers a file without a previously recorded
    offset.
-   `OUTPUT` uses the Elasticsearch plugin to send records to
    `elasticsearch:9200` on the Docker Compose network.
-   `Logstash_Format On` with `Logstash_Prefix payment-logs` creates
    date-suffixed indices such as `payment-logs-YYYY.MM.DD`.
-   `Retry_Limit False` configures retries without a fixed retry count,
    subject to process, buffering, and resource constraints.

Parser configuration used:

``` ini
[PARSER]
    Name        app_json
    Format      json
    Time_Key    time
    Time_Format %Y-%m-%dT%H:%M:%S.%L%z
```

The `[SERVICE]` section must load the parser file,
e.g. `Parsers_File /fluent-bit/etc/parsers.conf`. A previous
`parser 'app_json' is not registered` error was fixed by loading
`parsers.conf`.

## Useful commands

Run these from the project root.

### Start and inspect the stack

``` bash
sudo docker compose up -d
sudo docker compose ps
sudo docker logs --tail 100 fluent-bit
sudo docker logs --tail 100 elasticsearch
sudo docker logs --tail 100 kibana
```

Follow Fluent Bit logs:

``` bash
sudo docker logs -f fluent-bit
```

### Check Elasticsearch and indexed records

``` bash
curl -s http://localhost:9200
curl -s 'http://localhost:9200/payment-logs-*/_count?pretty'
curl -s 'http://localhost:9200/payment-logs-*/_search?pretty&size=20'
```

### Search in Kibana

1.  Open http://localhost:5601.
2.  Create a Data View named `payment-logs-*` if needed.
3.  Open **Discover** and select that Data View.
4.  Choose a time range that includes the log timestamps.
5.  Try KQL queries:

``` text
level: "ERROR"
payment_id: "pay-123"
service: "payment-service"
```

## Experiments completed

### 1. End-to-end pipeline

**Goal:** Confirm logs reach Kibana.

**Result:** The Go application wrote JSON to `app.log`; Fluent Bit read
and parsed it; Elasticsearch indexed it; Kibana displayed it.

**Lessons:** - Structured logs make filtering and correlation easier. -
Fluent Bit must load the parser configuration. - Bind mounts must point
to the actual file. A previous path mistake created a directory where a
log file was expected; this was corrected.

### 2. Elasticsearch outage and recovery

**Procedure:** Stop Elasticsearch, generate more application logs while
Fluent Bit stays running, inspect Fluent Bit retries, then start
Elasticsearch again.

``` bash
sudo docker compose stop elasticsearch
sudo docker logs -f fluent-bit
sudo docker compose start elasticsearch
curl -s http://localhost:9200
curl -s 'http://localhost:9200/payment-logs-*/_count?pretty'
```

**Observed result:** Fluent Bit logged failed flushes with retry delays.
After Elasticsearch recovered, Fluent Bit reported a successful flush,
and logs became searchable in Kibana.

**Lessons:** - Fluent Bit groups records into chunks. - Failed output
delivery triggers retries with backoff. - A temporary destination outage
does not necessarily cause permanent loss. - An indefinite retry policy
alone does not guarantee durable delivery.

### 3. Restart Fluent Bit during the Elasticsearch outage

**Goal:** Observe collector behavior when it restarts while the
destination is unavailable.

**Observed result:** After restart, Fluent Bit logged delivery failures
and retried a chunk. The chunk eventually flushed successfully after
Elasticsearch recovered. The original event timestamps appeared in
Kibana, and no duplicate was observed in the marker check.

**What remains unproven:** The logs do not conclusively establish
whether the successful chunk was recreated by rereading the source file
or came from another mechanism. Do not treat this as proof that
memory-buffered chunks survive process restarts.

## Important distinction: file, offset, and buffer

  -----------------------------------------------------------------------------
  Mechanism               What it stores          Current setup
  ----------------------- ----------------------- -----------------------------
  Application log file    Original records        `app.log` is on disk

  Tail offset tracking    Position read in the    No Tail `DB` configured
                          source file             

  Fluent Bit output       Records awaiting        Memory-based; logs reported
  buffer                  delivery                `storage_strategy='memory'`

  Elasticsearch index     Successfully indexed    `payment-logs-*`
                          documents               
  -----------------------------------------------------------------------------

`Read_from_Head true` does not mean the entire file is reread on every
restart. It applies when Tail discovers a file without a previously
recorded offset. With no Tail database configured, restart and
rediscovery behavior should be tested rather than assumed.

**Key takeaway:** A persistent source file is not the same as a
persistent collector buffer. A source file may allow records to be
reread, but that is not equivalent to durable buffering or guaranteed
exactly-once delivery.

## Operational lessons

-   **Retries are not durability.** Queued records are not guaranteed to
    survive a process or host failure just because retries are
    unlimited.
-   **Memory buffers have limits.** Long outages can cause memory
    pressure and affect collection.
-   **The source file and collector queue are separate.** Original
    records may remain in the file even if an in-memory queue is lost.
-   **Duplicate delivery is possible.** File rereading and retries
    should be considered in downstream design.
-   **Disk usage needs separate controls.** Application-file rotation,
    container-log rotation, Fluent Bit filesystem-buffer limits, and
    Elasticsearch index retention are different concerns.
-   **Kibana time filters matter.** A record may be indexed but hidden
    if its timestamp falls outside the selected time range.

## Next project: stdout/stderr and container logging

Target architecture:

``` text
Go application
      |
      | JSON to stdout/stderr
      v
Docker logging system
      |
      v
Fluent Bit
      |
      v
Elasticsearch
      |
      v
Kibana
```

Planned steps:

1.  Change the Go logger to write JSON to stdout/stderr instead of
    `app.log`.
2.  Run the application in a container.
3.  Inspect its output using `docker logs`.
4.  Configure Fluent Bit to collect container logs.
5.  Forward them to the existing Elasticsearch and Kibana setup.
6.  Test outages, collector restarts, and log rotation.

The exact collection mechanism depends on the chosen Docker logging
driver and Fluent Bit deployment. Docker logging drivers and Fluent Bit
are distinct components. Using stdout/stderr does not eliminate disk
usage or log-retention concerns.

## Suggested starting point for the next session

Inspect the logger initialization in `app/main.go`. Make the smallest
change needed to send JSON logs to stdout, preserving payment logic.
Verify the output locally before containerizing the application.
Continue incrementally: change one component, observe the result, then
proceed.
