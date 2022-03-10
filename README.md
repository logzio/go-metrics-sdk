# Ship Custom Metrics From Your GO Application

Create custom metrics in your Go application and ship them to Logz.io, 
using the exporter that sends cumulative metrics data from the OpenTelemetry
Go SDK to Logz.io using the Prometheus Remote Write API.

This exporter is push-based and integrates with the OpenTelemetry Go SDK's [Controller](https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/controller/basic/controller.go).
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
* [Setting up the Exporter](#setting-up-the-exporter)
* [Setting up the Metric Instruments Registry](#setting-up-the-metric-instruments-registry)
* [Metric Instrument to Aggregation Mapping](#metric-instrument-to-aggregation-mapping)
* [Metric Instrumentation and Recording Values](#metric-instrumentation-and-recording-values)
* [Error Handling](#error-handling)
* [Retry Logic](#retry-logic)
* [Full Example](#full-example)

## Installation

```bash
go get -u github.com/logzio/go-metrics-sdk
```

## Configuring the Exporter

The Exporter requires certain information, such as the Logz.io metrics listener URL, Logz.io metrics token and push interval
duration, to function properly. This information is stored in a `Config` struct, which is
passed into the Exporter during the setup pipeline.

Replace `<<LOGZIO_METRICS_LISTENER>>` with your Logz.io metrics listener URL.
Replace `<<LOGZIO_METRICS_TOKEN>>` with your Logz.io metrics token.

```go
import (
    metricsExporter "github.com/logzio/go-metrics-sdk"
    // ...
)

config := metricsExporter.Config {
	LogzioMetricsListener: "<<LOGZIO_METRICS_LISTENER>>",
	LogzioMetricsToken:    "<<LOGZIO_METRICS_TOKEN>>"
	RemoteTimeout:         30 * time.Second,
	PushInterval:          5 * time.Second,
}
```

Here is the `Config` struct definition.

```go
type Config struct {
	LogzioMetricsListener string
	LogzioMetricsToken    string
	RemoteTimeout         time.Duration
	PushInterval          time.Duration
	Quantiles             []float64
	HistogramBoundaries   []float64
}
```

| Parameter Name | Description | Required/Optional | Default |
| --- | --- | --- | --- |
| LogzioMetricsListener | The Logz.io metrics Listener URL for your region with port 8053. | Required | https://listener.logz.io:8053 |
| LogzioMetricsToken | The Logz.io metrics shipping token securely directs the data to your Logz.io account. | Required | - |
| RemoteTimeout | The timeout for requests to the remote write Logz.io metrics listener endpoint. | Required | 30 (seconds) |
| PushInterval | The time interval for sending the metrics to Logz.io. | Required | 10 (seconds) |
| Quantiles | The quantiles of the histograms. | Optional | [0.5, 0.9, 0.95, 0.99] |
| HistogramBoundaries | The histogram boundaries. | Optional | - |


## Setting up the Exporter

Call the `InstallNewPipeline` function to set up the exporter. It
requires a `Config` struct and returns a push Controller and error. If the error is nil,
the setup is successful and the user can begin creating instruments. No other action is
needed.

* Replace `<<COLLECT_PERIOD>>` with the collect period time (seconds).
* You can use `attribute.<<TYPE>>("<<LABEL_KEY>>", "<<LABEL_VALUE>>")` to add labels to all metric instruments. 
  You can add more than one `attribute` (make sure to replace `<<LABEL_KEY>>` and `<<LABEL_VALUE>>` 
  with you label's key and value accordingly, and `<<TYPE>>` with the available types according to the 
  `<<LABEL_VALUE>>` type you are using).

```go
// Use the `config` instance from last step.

cont, err := metricsExporter.InstallNewPipeline(
    config,
    controller.WithCollectPeriod(<<COLLECT_PERIOD>>*time.Second),
    controller.WithResource(
        resource.NewWithAttributes(
            semconv.SchemaURL,
            attribute.String("LABEL_KEY", "LABEL_VALUE"),
        ),
    ),
)
if err != nil {
    return err
}
```

## Setting up the Metric Instruments Creator

Create `Meter` to be able to create metric instruments.

Replace `<<INSTRUMENTATION_NAME>>` with your instrumentation name.

```go
// Use `cont` instance from last step.

ctx := context.Background()
defer func() {
    handleErr(cont.Stop(ctx))
}()

meter := cont.Meter("<<INSTRUMENTATION_NAME>>")
```

## Metric Instrument to Aggregation Mapping

The exporter uses the `simple` selector's `NewWithHistogramDistribution()`. This means
that instruments are mapped to aggregations as shown in the table below.

| Instrument | Behavior | Aggregation |
| --- | --- | --- |
| Counter | a synchronous Instrument which supports non-negative increments. | Sum |
| Asynchronous Counter | an asynchronous Instrument which reports monotonically increasing value(s) when the instrument is being observed. | Sum |
| Histogram | a synchronous Instrument which can be used to report arbitrary values that are likely to be statistically meaningful. It is intended for statistics such as histograms, summaries, and percentile. | Histogram |
| Asynchronous Gauge | an asynchronous Instrument which reports non-additive value(s) when the instrument is being observed. | LastValue |
| UpDownCounter | a synchronous Instrument which supports increments and decrements. | Sum |
| Asynchronous UpDownCounter | an asynchronous Instrument which reports additive value(s) when the instrument is being observed. | Sum |

For more information, see the OpenTelemetry [documentation](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/api.md).

## Metric Instrumentation and Recording Values

* You can use `attribute.<<TYPE>>("<<LABEL_KEY>>", "<<LABEL_VALUE>>")` to add labels to your 
  metric instruments. You can add more than one `attribute` (see explanation in last steps).

### Counter

```go
// Use `ctx` and `meter` from last steps.

// Create counter instruments
intCounter := metric.Must(meter).NewInt64Counter(
    "go_metrics.int_counter",
    metric.WithDescription("int_counter description"),
)
floatCounter := metric.Must(meter).NewFloat64Counter(
    "go_metrics.float_counter",
    metric.WithDescription("float_counter description"),
)

// Record values to the metric instruments and add labels
intCounter.Add(ctx, int64(10), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
floatCounter.Add(ctx, float64(2.5), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
```

### Asynchronous Counter

```go
// Use `meter` from last steps.

// Create callbacks for your CounterObserver instruments
intCounterObserverCallback := func(_ context.Context, result metric.Int64ObserverResult) {
    result.Observe(10, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
}
floatCounterObserverCallback := func(_ context.Context, result metric.Float64ObserverResult) {
    result.Observe(2.5, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
}

// Create CounterObserver instruments
_ = metric.Must(meter).NewInt64CounterObserver(
    "go_metrics.int_counter_observer",
    intCounterObserverCallback,
    metric.WithDescription("int_counter_observer description"),
)
_ = metric.Must(meter).NewFloat64CounterObserver(
    "go_metrics.float_counter_observer",
    floatCounterObserverCallback,
    metric.WithDescription("float_counter_observer description"),
)
```

### Histogram

```go
// Use `ctx` and `meter` from last steps.

// Create Histogram instruments
intHistogram := metric.Must(meter).NewInt64Histogram(
    "go_metrics.int_histogram",
    metric.WithDescription("int_histogram description"),
)
floatHistogram := metric.Must(meter).NewFloat64Histogram(
    "go_metrics.float_histogram",
    metric.WithDescription("float_histogram description"),
)

// Record values to the metric instruments and add labels
intHistogram.Record(ctx, int(10), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
floatHistogram.Record(ctx, float64(2.5), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
```

### Asynchronous Gauge

```go
// Use `meter` from last steps.

// Create callbacks for your GaugeObserver instruments
intGaugeObserverCallback := func(_ context.Context, result metric.Int64ObserverResult) {
    result.Observe(10, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
}
floatGaugeObserverCallback := func(_ context.Context, result metric.Float64ObserverResult) {
result.Observe(2.5, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>>"))
}

// Create GaugeObserver instruments
_ = metric.Must(meter).NewInt64GaugeObserver(
    "go_metrics.int_gauge_observer", 
    intGaugeObserverCallback,
    metric.WithDescription("int_gauge_observer description"),
)
_ = metric.Must(meter).NewFloat64GaugeObserver(
    "go_metrics.float_gauge_observer",
    floatGaugeObserverCallback,
    metric.WithDescription("float_gauge_observer description"),
)
```

### UpDownCounter

```go
// Use `ctx` and `meter` from last steps.

// Create UpDownCounter instruments
intUpDownCounter := metric.Must(meter).NewInt64UpDownCounter(
    "go_metrics.int_up_down_counter",
    metric.WithDescription("int_up_down_counter description"),
)
floatUpDownCounter := metric.Must(meter).NewFloat64UpDownCounter(
    "go_metrics.float_up_down_counter",
    metric.WithDescription("float_up_down_counter description"),
)

// Record values to the metric instruments and add labels
intUpDownCounter.Add(ctx, int64(-10), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
floatUpDownCounter.Add(ctx, float64(2.5), attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
```

### Asynchronous UpDownCounter

```go
// Use `meter` from last steps.

// Create callback for your UpDownCounterObserver instruments
intUpDownCounterObserverCallback := func(_ context.Context, result metric.Int64ObserverResult) {
    result.Observe(-10, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
}
floatUpDownCounterObserverCallback := func(_ context.Context, result metric.Float64ObserverResult) {
    result.Observe(2.5, attribute.String("<<LABEL_KEY>>", "<<LABEL_VALUE>"))
}

// Create UpDownCounterObserver instruments
_ = metric.Must(meter).NewInt64UpDownCounterObserver(
    "go_metrics.int_up_down_counter_observer",
    intUpDownCounterObserverCallback,
    metric.WithDescription("int_up_down_counter_observer description"),
)
_ = metric.Must(meter).NewFloat64UpDownCounterObserver(
    "go_metrics.float_up_down_counter_observer",
    floatUpDownCounterObserverCallback,
    metric.WithDescription("float_up_down_counter_observer description"),
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
package main

import (
    "context"
    "fmt"
    "math/rand"
    "time"

    metricsExporter "github.com/logzio/go-metrics-sdk"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

func main() {
    // Create Config struct.
    config := metricsExporter.Config{
        LogzioMetricsListener: "<<LOGZIO_METRICS_LISTENER>>",
        LogzioMetricsToken:    "<<LOGZIO_METRICS_TOKEN>>",
        RemoteTimeout:         30 * time.Second,
        PushInterval:          15 * time.Second,
    }

    // Create and install the exporter. Additionally, set the push interval to 5 seconds
    // and add a resource to the controller.
    cont, err := metricsExporter.InstallNewPipeline(
        config,
        controller.WithCollectPeriod(5*time.Second),
        controller.WithResource(
            resource.NewWithAttributes(
                semconv.SchemaURL,
                attribute.String("KEY", "VALUE"),
            ),
        ),
    )
    if err != nil {
        panic(fmt.Errorf("error: %v", err))
    }

    ctx := context.Background()
    defer func() {
        handleErr(cont.Stop(ctx))
    }()

    fmt.Println("Success: Installed Exporter Pipeline")

    // Create a counter and histogram
    meter := cont.Meter("example")

    // Create metric instruments
    histogram := metric.Must(meter).NewInt64Histogram(
        "example.histogram",
        metric.WithDescription("Records values"),
    )
    counter := metric.Must(meter).NewInt64Counter(
        "example.counter",
        metric.WithDescription("Counts things"),
    )

    fmt.Println("Success: Created Int64Histogram and Int64Counter instruments!")

    // Record random values to the metric instruments in a loop
    fmt.Println("Starting to write data to the metric instruments!")

    seed := rand.NewSource(time.Now().UnixNano())
    random := rand.New(seed)

    for {
        time.Sleep(1 * time.Second)

        randomValue := random.Intn(100)
        value := int64(randomValue * 10)

        histogram.Record(ctx, value, attribute.String("key", "value"))
        counter.Add(ctx, int64(randomValue), attribute.String("key", "value"))

        fmt.Printf("Adding %d to counter and recording %d in histogram\n", randomValue, value)
    }
}

func handleErr(err error) {
    if err != nil {
        panic(fmt.Errorf("encountered error: %v", err))
    }
}
```
