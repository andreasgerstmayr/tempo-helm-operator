# Generate traces
telemetrygen traces --traces=1 --otlp-insecure --otlp-endpoint=opentelemetry-collector.chainsaw-multitenancy.svc.cluster.local:4317

# Access Jaeger UI
https://sample-tempo-observatorium.chainsaw-multitenancy.svc.cluster.local:8080/tenant1
mail: foo@bar.com
password: foobar
select "openid" and click "Allow access"
