# Observability Lab

A hands-on lab for understanding structured application logging, log
collection, centralized storage, search, and failure recovery.

I will be using different approch in different branch

## Approach 1:

Go App → app.log → Fluent Bit → Elasticsearch → Kibana

## Approach 2:

Go App → stdout/stderr → Docker logging system → Log collector → Elasticsearch → Kibana