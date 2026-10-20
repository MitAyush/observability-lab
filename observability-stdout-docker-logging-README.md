# Observability Lab: Go stdout/stderr to Elasticsearch

## Goal

Explore how application logs travel through a containerized system,
become searchable, and behave during failures.

``` text
Go App → stdout/stderr → Docker logging system → Log collector → Elasticsearch → Kibana
```

## Architecture

``` text
Go payment service
  slog JSON → stdout/stderr
          |
          v
Docker logging system
  Fluentd logging driver
          | TCP / Forward protocol
          v
Fluent Bit
  receive, parse, buffer, forward
          |
          v
Elasticsearch
  index log documents
          |
          v
Kibana
  search and visualize
```

In this lab, Elasticsearch and Kibana run in Docker Compose. Fluent Bit
runs in a container and exposes port `24224`. The `payment-service`
container is launched separately and uses Docker's Fluentd logging
driver.

## 1. Application logging

The Go app uses `log/slog` with a JSON handler writing to stdout:

``` go
logger := slog.New(
    slog.NewJSONHandler(os.Stdout, nil),
)
```

Example application event:

``` json
{
  "time": "2026-10-20T09:16:35Z",
  "level": "INFO",
  "msg": "payment processing started",
  "service": "payment-service",
  "payment_id": "pay-123",
  "correlation_id": "example-request-id"
}
```

The timestamp and correlation ID vary between runs.

Writing to stdout/stderr lets Docker's logging system handle container
output instead of requiring the application to manage its own log file.

## 2. Collection approach A: Tail Docker's `json-file` logs

The initial experiment used Docker's `json-file` logging driver. Fluent
Bit tailed files such as:

``` text
/var/lib/docker/containers/<container-id>/<container-id>-json.log
```

Example input:

``` ini
[INPUT]
    Name              tail
    Path              /var/lib/docker/containers/*/*-json.log
    Tag               docker.*
    Parser            docker
    Read_from_Head    true
    Refresh_Interval  5
```

The wildcard avoids hardcoding container IDs, but collects matching logs
from other containers too. Reading Docker's internal log directory
directly is useful for a local lab; production choices depend on the
runtime and platform.

## 3. Collection approach B: Docker Fluentd logging driver

The lab then switched to Docker's Fluentd logging driver. This avoids
hardcoding a container ID and avoids tailing Docker's internal log files
for this application.

Fluent Bit needs a Forward input:

``` ini
[INPUT]
    Name    forward
    Listen  0.0.0.0
    Port    24224
```

The Fluent Bit Compose service publishes the port:

``` yaml
ports:
  - "24224:24224"
```

The host port lets the Docker daemon connect to Fluent Bit. The daemon's
`localhost` is the host network namespace, not the payment-service
container's namespace. Docker port publishing forwards the host
connection to the Fluent Bit container.

Launch the app with:

``` bash
docker run --rm \
  --name payment-service \
  --log-driver=fluentd \
  --log-opt fluentd-address=localhost:24224 \
  --log-opt tag=docker.payment-service \
  payment-service:latest
```

Assumptions: - Docker and the published port are on the same host. -
Fluent Bit listens on `0.0.0.0:24224` inside its container. - Port
`24224` is published and reachable. - The image is named
`payment-service:latest`.

Useful diagnostics:

``` bash
docker inspect payment-service \
  --format '{{json .HostConfig.LogConfig}}'

docker compose logs --tail=50 fluent-bit

docker port fluent-bit 24224
```

## 4. Parsing structured logs

In the file-tail approach, Docker's `json-file` driver wraps application
output in an outer JSON object. The application JSON may initially
remain a string inside its `log` field. The pipeline therefore used two
parsing stages:

1.  Parse the outer Docker JSON envelope.
2.  Parse the application JSON inside `log`.

With the Fluentd driver and Forward input, don't assume the incoming
record has the same envelope. Inspect records and configure parsing for
the actual input. Do not blindly retain a parser filter that expects the
old Docker JSON envelope.

The goal is for fields such as `service`, `level`, `payment_id`, and
`correlation_id` to be individually searchable in Elasticsearch. Parsing
changes apply to new records; existing indexed documents are not
transformed retroactively.

## 5. Elasticsearch and Kibana

The local lab uses Elasticsearch and Kibana `8.17.0`. Elasticsearch
security has been disabled for this local experiment only.

-   Elasticsearch: `http://localhost:9200`
-   Kibana: `http://localhost:5601`
-   Data view used in Kibana: `payment-logs-*`

List indices:

``` bash
curl -s "http://localhost:9200/_cat/indices?v"
```

Search for payment-service records:

``` bash
curl -s "http://localhost:9200/payment-logs-*/_search?pretty" \
  -H 'Content-Type: application/json' \
  -d '{
    "size": 5,
    "query": {
      "match": {
        "service": "payment-service"
      }
    },
    "sort": [
      {
        "@timestamp": {
          "order": "desc"
        }
      }
    ]
  }'
```

Quote URLs containing `?` in zsh to avoid shell glob expansion.

## 6. Failure experiment: Fluent Bit is stopped

The Go application was given a 10-second sleep before emitting its first
log. This allowed Fluent Bit to be stopped before the application
attempted to log.

Observed result: with the current synchronous/default Docker Fluentd
logging-driver behavior, the application became stuck while attempting
to deliver a log. This shows that the logging path can affect
application execution.

If logging blocks inside a request handler, it can increase latency or
consume application resources. Exact behavior depends on the logging
driver, connection state, and timeout configuration.

### Experiments to run next

Run one change at a time and record the actual results.

1.  **Collector unavailable at startup:** Check whether the app
    container starts with the current settings.
2.  **Collector unavailable during execution:** Observe whether log
    writes block, retry, or drop records.
3.  **Asynchronous connection handling:** Try
    `--log-opt fluentd-async=true` and compare startup and delivery
    behavior.
4.  **Non-blocking Docker logging:** Try `--log-opt mode=non-blocking`
    and `--log-opt max-buffer-size=4m`. Observe the trade-off between
    application progress and possible log loss.
5.  **Elasticsearch unavailable:** Stop Elasticsearch while Fluent Bit
    stays up. Observe Fluent Bit's buffering and retry behavior. This is
    a different failure boundary from Docker-to-Fluent-Bit delivery.

Do not assume asynchronous mode guarantees delivery. Buffers are finite.
Verify outcomes using application behavior, Fluent Bit logs, and
Elasticsearch queries.

For experiments that require post-run inspection, temporarily omit
`--rm` so the exited container remains available.

## 7. Concepts learned

-   **Application logging:** The app emits structured logs to
    stdout/stderr.
-   **Container logging:** Docker's logging driver handles container
    output.
-   **Collection:** Fluent Bit tails log files or receives forwarded
    records.
-   **Parsing:** JSON strings must be parsed to make their fields
    independently searchable.
-   **Buffering and retries:** These affect behavior during downstream
    outages.
-   **Backpressure:** A blocked log write can affect application
    execution.
-   **Loss versus availability:** Non-blocking logging can protect
    application progress but may drop logs when buffers fill.
-   **Container lifecycle:** `--rm` removes the container after exit; it
    does not remove records already indexed in Elasticsearch.
-   **Dynamic discovery:** Avoid tying collection to a hardcoded
    container ID when containers can be recreated.

## 8. Current status

-   [x] Go app writes structured JSON logs to stdout.
-   [x] Docker captures application output.
-   [x] Fluent Bit receives logs through Docker's Fluentd logging
    driver.
-   [x] Logs reach Elasticsearch and appear in Kibana.
-   [x] Observed the application blocking when Fluent Bit was
    unavailable during log delivery.
-   [ ] Compare synchronous and asynchronous behavior.
-   [ ] Test non-blocking mode and bounded buffers.
-   [ ] Test an Elasticsearch outage separately.
-   [ ] Record which logs are delayed or lost in each experiment.

## Production considerations

This is a local learning lab, not a production-ready deployment.
Production designs should consider authentication and TLS, network
restrictions, resource limits, log rotation, collector restarts,
buffering limits, delivery guarantees, duplicate records, retention,
monitoring of the logging pipeline, and the orchestration platform's
recommended collection method.
