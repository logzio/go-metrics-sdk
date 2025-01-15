# Ship Custom Metrics From Your GO Application

Create custom metrics in your Go application and ship them to Logz.io, 
using the exporter that sends cumulative metrics data from the OpenTelemetry
Go SDK to Logz.io using the Prometheus Remote Write API.

This exporter integrates with the OpenTelemetry Go SDK's [Controller](https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/controller/basic/controller.go).
The Controller periodically collects data and passes it to this exporter. The exporter
then converts this data into
[`TimeSeries`](https://prometheus.io/docs/concepts/data_model/), a format that Logz.io
accepts, and sends it to Logz.io through HTTP POST requests. The request body is formatted
according to the protocol defined by the Prometheus Remote Write API. See Prometheus's
[remote storage integration
documentation](https://prometheus.io/docs/prometheus/latest/storage/#remote-storage-integrations)
for more details on the Remote Write API.

## Table of Contents

* [Installation](#installation)
* [Configuring the Exporter](#configuring-the-exporter)
* [Setting up the Metric Instruments Registry](#setting-up-the-metric-instruments-registry)
* [Metric Instrument to Aggregation Mapping](#metric-instrument-to-aggregation-mapping)
* [Metric Instrumentation and Recording Values](#metric-instrumentation-and-recording-values)
* [Error Handling](#error-handling)
* [Retry Logic](#retry-logic)
* [Full Example](#full-example)

## Installation

```bash
go get github.com/logzio/go-metrics-sdk
```

## Configuring the Exporter

The Exporter requires certain information, such as the Logz.io metrics listener URL, Logz.io metrics token and push interval
duration, to function properly. This information is stored in a `Config` struct, which is
passed into the Exporter during the setup pipeline.

Example:

```go
import (
    "context"
    metricsExporter "github.com/logzio/go-metrics-sdk"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    m "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    "log"
    "time"
)

func newLogzioExporter() (*metricsExporter.Exporter, error) {
    return metricsExporter.New(
        metricsExporter.Config{
            LogzioMetricsListener: "https://<<LOGZIO_METRICS_LISTENER>>:8053",
            LogzioMetricsToken:    "<<LOGZIO_METRICS_TOKEN>>",
            RemoteTimeout:         30 * time.Second,
            PushInterval:          10 * time.Second,
            Quantiles:             []float64{0, 0.25, 0.5, 0.75, 1},
            AddMetricSuffixes: true,
            ExternalLabels: map[string]string{
                "<<LABEL_KEY>>": "<<LABEL_VALUE>>",
            },
        })
}
```

- Replace `<<LOGZIO_METRICS_LISTENER>>` with your Logz.io metrics listener URL.
- Replace `<<LOGZIO_METRICS_TOKEN>>` with your Logz.io metrics token.
- Replace `<<LABEL_KEY>>` and `<<LABEL_VALUE>>` with a label you want to apply to all metrics. You can add more labels if needed or remove the `ExternalLabels` section entirely if you don't want to add any global labels.


### Config Struct all options

```go
type Config struct {
	LogzioMetricsListener string
	LogzioMetricsToken    string
	RemoteTimeout         time.Duration
	PushInterval          time.Duration
	Quantiles             []float64
	HistogramBoundaries   []float64
	ExternalLabels        map[string]string
	AddMetricSuffixes     bool
}
```

| Parameter Name        | Description                                                                           | Required/Optional | Default                       |
|-----------------------|---------------------------------------------------------------------------------------|-------------------|-------------------------------|
| LogzioMetricsListener | The Logz.io metrics Listener URL for your region with port 8053.                      | Required          | https://listener.logz.io:8053 |
| LogzioMetricsToken    | The Logz.io metrics shipping token securely directs the data to your Logz.io account. | Required          | -                             |
| RemoteTimeout         | The timeout for requests to the remote write Logz.io metrics listener endpoint.       | Required          | 30 (seconds)                  |
| PushInterval          | The time interval for sending the metrics to Logz.io.                                 | Required          | 10 (seconds)                  |
| Quantiles             | The quantiles of the histograms.                                                      | Optional          | [0.5, 0.9, 0.95, 0.99]        |
| HistogramBoundaries   | The histogram boundaries.                                                             | Optional          | -                             |
| ExternalLabels        | Allow adding global labels to all metrics that are processed by the exporter.         | Optional          | -                             |
| AddMetricSuffixes     | Adds Unit suffix to the metric, if the Unit was defined.                              | Optional          | `false`                       |

## Setting up the Metric Instruments Creator

Create `Meter` to be able to create metric instruments.

```go
func newResource() (*resource.Resource, error) {
    return resource.Merge(resource.Default(),
        resource.NewWithAttributes(semconv.SchemaURL,
            semconv.ServiceName("<<SERVICE_NAME>>"),
            semconv.ServiceVersion("<<SERVICE_VERSION>>"),
        ))
}

func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
    metricExporter, err := newLogzioExporter()
	if err != nil {
        return nil, err
    }

    meterProvider := metric.NewMeterProvider(
        metric.WithResource(res),
        metric.WithReader(metric.NewPeriodicReader(metricExporter,
        metric.WithInterval(60*time.Second))),
    )
    return meterProvider, nil
}

// create a context
con := context.Background()

// Create resource
res, err := newResource()
if err != nil {
    panic(err)
}

// Create a meter provider.
meterProvider, err := newMeterProvider(res)
if err != nil {
	panic(err)
}

// Handle shutdown properly so nothing leaks.
defer func() {
    if err := meterProvider.Shutdown(con); err != nil {
        log.Println(err)
    }
}()

// Register as global meter provider so that it can be used via otel.Meter
// and accessed using otel.GetMeterProvider.
// Most instrumentation libraries use the global meter provider as default.
// If the global meter provider is not set then a no-op implementation
// is used, which fails to generate data.
otel.SetMeterProvider(meterProvider)

meter := otel.Meter("example-meter")  // replace `example-meter` with any custom instrumentation meter name you'd like
```

## Metric Instrument to Aggregation Mapping

The exporter uses the `simple` selector's `metric.DefaultAggregationSelector()`. This means
that instruments are mapped to aggregations as shown in the table below.

| Instrument                 | Behavior                                                                                                                                                                                           | Aggregation |
|----------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| Counter                    | a synchronous Instrument which supports non-negative increments.                                                                                                                                   | Sum         |
| Asynchronous Counter       | an asynchronous Instrument which reports monotonically increasing value(s) when the instrument is being observed.                                                                                  | Sum         |
| Histogram                  | a synchronous Instrument which can be used to report arbitrary values that are likely to be statistically meaningful. It is intended for statistics such as histograms, summaries, and percentile. | Histogram   |
| Asynchronous Gauge         | an asynchronous Instrument which reports non-additive value(s) when the instrument is being observed.                                                                                              | Gauge       |
| UpDownCounter              | a synchronous Instrument which supports increments and decrements.                                                                                                                                 | Sum         |
| Asynchronous UpDownCounter | an asynchronous Instrument which reports additive value(s) when the instrument is being observed.                                                                                                  | Sum         |

For more information, see the OpenTelemetry [documentation](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/api.md).

## Metric Instrumentation and Recording Values

* You can use `attribute.<<TYPE>>("<<LABEL_KEY>>", "<<LABEL_VALUE>>")` to add labels to your 
  metric instruments. You can add more than one `attribute` (see explanation in last steps).

### Counter

```go
// Create counter instruments
counter, err := meter.Int64Counter(
    "int_counter",  // name of the metric
    // m.WithUnit("ms")  // add unit to the metric if you want
    m.WithDescription("Counts the total number of requests"),
)

floatCounter, err := meter.Float64Counter("float_counter")  // name of the metric

// Record values to the metric instruments and add labels
counter.Add(con, 1)
counter.Add(con, 10, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
floatCounter.Add(con, 2.5)
```

### Asynchronous Counter

```go
// Create callbacks for your observable counter instruments
// Float64ObservableCounter is also a supported type
	observableCounter, err := meter.Int64ObservableCounter(
        "observable_counter",  // name of the metric
        // m.WithUnit("By"),  // add unit to the metric if you want
        m.WithDescription("observable counter description"),
)

_, err = meter.RegisterCallback(
    func(con context.Context, o m.Observer) error {
        o.ObserveInt64(observableCounter, 10, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
        return nil
    },
    observableCounter,
)
```

### Histogram

```go
// Create Histogram instruments
intHistogram, err := meter.Int64Histogram(
	"histogram-meter",  // name of the metric
    // m.WithUnit("seconds")  // add unit to the metric if you want
    m.WithDescription("Histogram description"),
)

floatHistogram, err := meter.Float64Histogram("float-histogram-meter")  // name of the metric

// Record values to the metric instruments and add labels
intHistogram.Record(con, 2)
intHistogram.Record(con, 2, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
floatHistogram.Record(con, 3.4)
```

### Gauge

```go
g, err := meter.Int64Gauge(
	"g-test",
	// m.WithUnit("seconds"),  // add unit to the metric if you want
    m.WithDescription("Gauge description"),
)

g.Record(con, 4)
g.Record(con, 4, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
```

### Asynchronous Gauge

```go
// Create callbacks for your observable up-down counter instruments
// Float64ObservableGauge is also a supported type
observableGauge, err := meter.Int64ObservableGauge(
    "observable_gauge",  // name of the metric
    // m.WithUnit("By"),  // add unit to the metric if you want
    m.WithDescription("observable gauge description"),
)

_, err = meter.RegisterCallback(
    func(con context.Context, o m.Observer) error {
        o.ObserveInt64(observableGauge, 10, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
        return nil
    },
observableGauge,
)
```

### UpDownCounter

```go
intUpDownCounter, err := meter.Int64UpDownCounter(
    "int_up_down_counter",  // name of the metric
    // m.WithUnit("ms"),  // add unit to the metric if you want
    m.WithDescription("Up-down counter description"),
)

floatUpDownCounter, err := meter.Float64UpDownCounter(
    "float_up_down_counter",  // name of the metric
    m.WithDescription("Up-down counter description"),
)

intUpDownCounter.Add(con, 5)
floatUpDownCounter.Add(con, 5.6, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
```

### Asynchronous UpDownCounter

```go
// Create callbacks for your observable up-down counter instruments
// Float64ObservableUpDownCounter is also a supported type
observableCounter, err := meter.Int64ObservableUpDownCounter(
    "observable_up_down_counter",  // name of the metric
    // m.WithUnit("By"),  // add unit to the metric if you want
    m.WithDescription("observable up down counter description"),
)

_, err = meter.RegisterCallback(
    func(con context.Context, o m.Observer) error {
        o.ObserveInt64(observableCounter, 10, m.WithAttributes(attribute.Key("<<LABEL_KEY>>").String("<<LABEL_VALUE>>")))
        return nil
    },
    observableCounter,
)
```

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

```text
SUCCESS FAIL FAIL SUCCESS SUCCESS
1       2    3    4       5
```

Then the received data will be:

```text
1 4 5
```

The end result is the same since the aggregations are cumulative.

## Full Example

```go
package testtt

import (
	"context"
	metricsExporter "github.com/logzio/go-metrics-sdk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	m "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"log"
	"time"
)

func newLogzioExporter() (*metricsExporter.Exporter, error) {
	return metricsExporter.New(
		metricsExporter.Config{
			LogzioMetricsListener: "https://listener.logz.io:8053",
			LogzioMetricsToken:    "<<LOGZIO_METRICS_TOKEN>>",
			RemoteTimeout:         30 * time.Second,
			PushInterval:          10 * time.Second,
			Quantiles:             []float64{0, 0.25, 0.5, 0.75, 1},
            AddMetricSuffixes: true,
            ExternalLabels: map[string]string{
              "<<LABEL_KEY>>": "<<LABEL_VALUE>>",
            },
		})
}

func newResource() (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("my-service"),
			semconv.ServiceVersion("0.1.0"),
		))
}

func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
	metricExporter, err := newLogzioExporter()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
            // Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)
	return meterProvider, nil
}

func main() error {
  con := context.Background()
  res, err := newResource()
  if err != nil {
    panic(err)
  }

  meterProvider, err := newMeterProvider(res)
  if err != nil {
    panic(err)
  }

  defer func() {
    if err := meterProvider.Shutdown(con); err != nil {
      log.Println(err)
    }
  }()

  otel.SetMeterProvider(meterProvider)
  meter := otel.Meter("example-meter")

  counter, err := meter.Int64Counter(
    "requests_total",
    m.WithDescription("Counts the total number of requests"),
  )
  if err != nil {
    return err
  }
  
  counter.Add(con, 1)
  counter.Add(con, 10, m.WithAttributes(attribute.Key("metricLabel").String("val")))
}
```
