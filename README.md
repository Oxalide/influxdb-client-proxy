# influxdb-client-proxy

Tiny proxy to enforce a filter on InfluxQL queries.

## How to use

Enable the InfluxDB HTTP API. For this example, we'll assume it is reachable at `influx-node:8086`.

To scope a Grafana organization to the client named `test-client`, configure an InfluxDB datasource to hit the proxy (default listening port: 9094).
Do *not* allow the client to directly hit it. Every request must use the Grafana proxy first, which ensures the client is correctly authenticated.

Append to the datasource URL the client name, making it in our case `http://localhost:9094/test-client`.

Now, every request made through Grafana will append the correct filters.
