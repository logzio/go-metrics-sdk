# OpenTelemetry Go SDK Prometheus Remote Write Exporter for Logz.io

Exporter that sends cumulative metrics data from the OpenTelemetry
Go SDK to Logz.io using the Prometheus Remote Write API.

This exporter is push-based and integrates with the OpenTelemetry Go SDK's [push
Controller](https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/controller/push/push.go).
The Controller periodically collects data and passes it to this exporter. The exporter
then converts this data into
[`TimeSeries`](https://prometheus.io/docs/concepts/data_model/), a format that Logz.io
accepts, and sends it to Logz.io through HTTP POST requests. The request body is formatted
according to the protocol defined by the Prometheus Remote Write API. See Prometheus's
[remote storage integration
documentation](https://prometheus.io/docs/prometheus/latest/storage/#remote-storage-integrations)
for more details on the Remote Write API.

See the `example` submodule for a working example of this exporter.

Table of Contents
=================
   * [OpenTelemetry Go SDK Prometheus Remote Write Exporter for Logz.io](#opentelemetry-go-sdk-prometheus-remote-write-exporter-for-logzio)
   * [Table of Contents](#table-of-contents)
      * [Installation](#installation)
      * [Setting up the Exporter](#setting-up-the-exporter)
      * [Configuring the Exporter](#configuring-the-exporter)
      * [Instrument to Aggregation Mapping](#instrument-to-aggregation-mapping)
      * [Error Handling](#error-handling)
      * [Retry Logic](#retry-logic)

## Installation

```bash
go get -u github.com/logzio/go-metrics-sdk
```

## Setting up the Exporter

Users only need to call the `InstallNewPipeline` function to setup the exporter. It
requires a `Config` struct and returns a push Controller and error. If the error is nil,
the setup is successful and the user can begin creating instruments. No other action is
needed.

```go
import (
    metricsExporter "github.com/logzio/go-metrics-sdk"
)

// Create a Config struct named `config`.

controller, err := metricsExporter.InstallNewPipeline(config)
if err != nil {
    return err
}
defer controller.Stop(context.Background())

// Make instruments and record data using `global.MeterProvider`.
```

## Configuring the Exporter

The Exporter requires certain information, such as the Logz.io metrics listener URL, Logz.io metrics token and push interval
duration, to function properly. This information is stored in a `Config` struct, which is
passed into the Exporter during the setup pipeline.

```go
config := metricsExporter.Config {
  Endpoint:      "http://localhost:9009/api/prom/push",
	RemoteTimeout: 30 * time.Second,
	PushInterval: 5 * time.Second,
	Headers: map[string]string{
		"test": "header",
	},
}
// Validate() should be called when creating the Config struct manually.
if err := config.Validate(); err != nil {
	return err
}
```

Here is the `Config` struct definition.

```go
type Config struct {
	LogzioMetricsListener string
	LogzioMetricsToken    string
	RemoteTimeout         time.Duration
	PushInterval          time.Duration
	HistogramBoundaries   []float64
}
```

## Instrument to Aggregation Mapping

The exporter uses the `simple` selector's `NewWithHistogramDistribution()`. This means
that instruments are mapped to aggregations as shown in the table below.

| Instrument        | Aggregation |
|-------------------|-------------|
| Counter           | Sum         |
| UpDownCounter     | Sum         |
| ValueRecorder     | Histogram   |
| SumObserver       | Sum         |
| UpDownSumObserver | Sum         |
| ValueObserver     | Histogram   |

Although only the `Sum` and `Histogram` aggregations are currently being used, the
exporter supports 4 different aggregations:

1. `Sum`
2. `LastValue`
3. `Distribution`
4. `Histogram`

## Error Handling

In general, errors are returned to the calling function / method. Eventually, errors make
their way up to the push Controller where it calls the exporter's `Export()` method. The
push Controller passes the errors to the OpenTelemetry Go SDK's global error handler. 

The exception is when the exporter fails to send an HTTP request to Logz.io. Regardless of
status code, the error is ignored. See the retry logic section below for more details.

## Retry Logic

The exporter does not implement any retry logic since the exporter sends cumulative
metrics data, which means that data will be preserved even if some exports fail. 

For example, consider a situation where a user increments a `Counter` instrument 5 times
and an export happens between each increment. If the exports happen like so:

```
  SUCCESS FAIL FAIL SUCCESS SUCCESS
  1       2    3    4       5
```

Then the received data will be:

```
1 4 5
```

The end result is the same since the aggregations are cumulative.
