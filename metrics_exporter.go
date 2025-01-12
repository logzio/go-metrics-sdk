// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package metrics_exporter provides functionality to send cumulative metric data
// using the Prometheus Remote Write API to Logz.io.
package metrics_exporter

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"maps"
	"sync"

	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"net/http"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	traceIdLabelName          = "trace_id"
	spanIdLabelName           = "span_id"
	histogramSumSuffix        = "_sum"
	histogramMaxSuffix        = "_max"
	histogramMinSuffix        = "_min"
	histogramCountSuffix      = "_count"
	histogramLastBucketSuffix = "+inf" // Default for the last bucket
)

// Exporter forwards metrics to Logz.io
type Exporter struct {
	clientMu     sync.Mutex
	config       Config
	shutdownOnce sync.Once
}

// New returns a Logzio Prometheus remote write Exporter.
func New(config Config) (*Exporter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	exporter := Exporter{config: config}
	return &exporter, nil
}

// Temporality returns CumulativeExporter so the Processor correctly aggregates data
func (e *Exporter) Temporality(_ metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

// Export forwards metrics to Logz.io from the SDK
func (e *Exporter) Export(_ context.Context, rm *metricdata.ResourceMetrics) error {
	timeseries, err := e.ConvertToTimeSeries(rm)
	if err != nil {
		return err
	}

	message, buildMessageErr := e.buildMessage(timeseries)
	if buildMessageErr != nil {
		return buildMessageErr
	}

	request, buildRequestErr := e.buildRequest(message)
	if buildRequestErr != nil {
		return buildRequestErr
	}

	e.clientMu.Lock()
	sendRequestErr := e.sendRequest(request)
	e.clientMu.Unlock()
	if sendRequestErr != nil {
		return sendRequestErr
	}

	return nil
}

// ConvertToTimeSeries converts a InstrumentationLibraryReader to a slice of TimeSeries pointers
// Based on the aggregation type, ConvertToTimeSeries will call helper functions like
// convertFromSum to generate the correct number of TimeSeries.
func (e *Exporter) ConvertToTimeSeries(rm *metricdata.ResourceMetrics) ([]prompb.TimeSeries, error) {
	var timeSeries []prompb.TimeSeries
	var result *multierror.Error

	labelsMap := generateGlobalLabels(rm.Resource, e.config.ExternalLabels)

	// Iterate over each record in the checkpoint set and convert to TimeSeries
	for _, sm := range rm.ScopeMetrics {
		maps.Copy(labelsMap, generateScopeLabels(sm.Scope))

		for _, m := range sm.Metrics {
			metricName := m.Name
			if e.config.AddMetricSuffixes && m.Unit != "" {
				metricName = metricName + "_" + m.Unit
			}

			switch data := m.Data.(type) {
			case metricdata.Sum[int64]:
				ts, err := convertFromSum(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			case metricdata.Sum[float64]:
				ts, err := convertFromSum(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			case metricdata.Gauge[int64]:
				ts, err := convertFromGauge(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			case metricdata.Gauge[float64]:
				ts, err := convertFromGauge(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			case metricdata.Histogram[int64]:
				ts, err := convertFromHistogram(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			case metricdata.Histogram[float64]:
				ts, err := convertFromHistogram(metricName, data, labelsMap)
				if err != nil {
					result = multierror.Append(result, err)
				} else {
					timeSeries = append(timeSeries, ts...)
				}
			default:
				result = multierror.Append(result, fmt.Errorf("Unsupported metric type: %T\n", data))
			}
		}
	}

	return timeSeries, result.ErrorOrNil()
}

// createTimeSeries is a helper function to create a timeseries from a value and attributes
func createTimeSeries(value float64, ts time.Time, labels map[string]string, exemplars []prompb.Exemplar) prompb.TimeSeries {
	// We generate a sample per datapoint, because OTEL handles merging of datapoint with the same labels and name.
	// Therefore, if there are multiple data points >> they necessarily have different Attributes >> meaning they are
	// different timeseries.
	sample := prompb.Sample{
		Value:     value,
		Timestamp: ts.UnixNano() / int64(time.Millisecond),
	}
	return prompb.TimeSeries{
		Samples:   []prompb.Sample{sample},
		Labels:    createLabelSet(labels),
		Exemplars: exemplars,
	}
}

// convertFromSum returns a single TimeSeries based on a Record with a Sum aggregation
func convertFromSum[N int64 | float64](metricName string, sum metricdata.Sum[N], labels map[string]string) ([]prompb.TimeSeries, error) {
	var timeSeries []prompb.TimeSeries
	var dpLabels map[string]string

	for _, dp := range sum.DataPoints {
		var ex []prompb.Exemplar
		dpLabels = generateDataPointLabels(metricName, labels, dp.Attributes)
		// sum.IsMonotonic is true for prometheus.CounterValue, false for prometheus.GaugeValue
		// GaugeValues don't support Exemplars at this time
		// ref: https://github.com/prometheus/client_golang/blob/aef8aedb4b6e1fb8ac1c90790645169125594096/prometheus/metric.go#L199
		if sum.IsMonotonic {
			ex = generateExamplers(dp.Exemplars)
		}

		// we take the Time and not StartTime, because the Timestamp should be the time when the datapoint was recorded
		timeSeries = append(timeSeries, createTimeSeries(float64(dp.Value), dp.Time, dpLabels, ex))
	}

	return timeSeries, nil
}

// convertFromGauge returns a TimeSeries based on a Record with a Gauge aggregation
func convertFromGauge[N int64 | float64](metricName string, gauge metricdata.Gauge[N], labels map[string]string) ([]prompb.TimeSeries, error) {
	var timeSeries []prompb.TimeSeries
	var dpLabels map[string]string

	for _, dp := range gauge.DataPoints {
		dpLabels = generateDataPointLabels(metricName, labels, dp.Attributes)

		// GaugeValues don't support Exemplars at this time
		// ref: https://github.com/prometheus/client_golang/blob/aef8aedb4b6e1fb8ac1c90790645169125594096/prometheus/metric.go#L199
		// also, we take the Time and not StartTime, because the Timestamp should be the time when the datapoint was recorded
		timeSeries = append(timeSeries, createTimeSeries(float64(dp.Value), dp.Time, dpLabels, nil))
	}
	return timeSeries, nil
}

// convertFromHistogram returns len(histogram.Buckets) timeseries for a histogram aggregation
func convertFromHistogram[N int64 | float64](metricName string, histogram metricdata.Histogram[N], labels map[string]string) ([]prompb.TimeSeries, error) {
	var timeSeries []prompb.TimeSeries
	var totalCount float64

	for _, dp := range histogram.DataPoints {
		ex := generateExamplers(dp.Exemplars)

		// configure labels for each datapoint
		maxDpLabels := generateDataPointLabels(metricName+histogramMaxSuffix, labels, dp.Attributes)
		minDpLabels := generateDataPointLabels(metricName+histogramMinSuffix, labels, dp.Attributes)
		sumDpLabels := generateDataPointLabels(metricName+histogramSumSuffix, labels, dp.Attributes)
		countDpLabels := generateDataPointLabels(metricName+histogramCountSuffix, labels, dp.Attributes)
		boundDpLabels := generateDataPointLabels(metricName, labels, dp.Attributes)

		// add time series for each datapoint
		if maxVal, defined := dp.Max.Value(); defined {
			timeSeries = append(timeSeries, createTimeSeries(float64(maxVal), dp.Time, maxDpLabels, ex))
		}
		if minVal, defined := dp.Min.Value(); defined {
			timeSeries = append(timeSeries, createTimeSeries(float64(minVal), dp.Time, minDpLabels, ex))
		}
		timeSeries = append(timeSeries, createTimeSeries(float64(dp.Sum), dp.Time, sumDpLabels, ex))
		timeSeries = append(timeSeries, createTimeSeries(float64(dp.Count), dp.Time, countDpLabels, ex))

		// Handle histogram buckets
		for i, bucketCount := range dp.BucketCounts {
			boundDpLabels["le"] = fmt.Sprintf("%g", dp.Bounds[i])
			totalCount += float64(dp.BucketCounts[i])

			// Create timeseries for the bucket
			timeSeries = append(timeSeries, createTimeSeries(float64(bucketCount), dp.Time, boundDpLabels, ex))
		}
		boundDpLabels["le"] = histogramLastBucketSuffix
		timeSeries = append(timeSeries, createTimeSeries(totalCount, dp.Time, boundDpLabels, ex))
	}

	return timeSeries, nil
}

// generateGlobalLabels returns global labels to add to all metrics based on the resource and the exporter settings
func generateGlobalLabels(res *resource.Resource, exporterLabels map[string]string) map[string]string {
	globalLabels := map[string]string{}

	for _, attr := range res.Attributes() {
		globalLabels[string(attr.Key)] = attr.Value.Emit()
	}
	maps.Copy(globalLabels, exporterLabels)
	return globalLabels
}

// generateScopeLabels returns labels to add to a metric based on the scope and its attributes
func generateScopeLabels(scope instrumentation.Scope) map[string]string {
	scopeLabels := map[string]string{
		"otel_scope_name":    scope.Name,
		"otel_scope_version": scope.Version,
	}
	maps.Copy(scopeLabels, generateAttributesLabels(scope.Attributes))
	return scopeLabels
}

// generateAttributesLabels returns a map of labels from a set of attributes
func generateAttributesLabels(as attribute.Set) map[string]string {
	labels := map[string]string{}

	for _, attr := range as.ToSlice() {
		labels[string(attr.Key)] = attr.Value.Emit()
	}
	return labels
}

// addMetricName adds the metric name as attribute to the timeseries as Logz.io requires it.
func addMetricName(metricName string, labels map[string]string) map[string]string {
	result := map[string]string{}
	maps.Copy(result, labels)
	result["__name__"] = metricName
	return result
}

// generateExamplers returns a slice of prompb.Exemplar from a slice of metricdata.Exemplar
func generateExamplers[N int64 | float64](exemplars []metricdata.Exemplar[N]) []prompb.Exemplar {
	labels := map[string]string{}
	result := make([]prompb.Exemplar, 0, len(exemplars))
	for i, ex := range exemplars {
		labels[traceIdLabelName] = hex.EncodeToString(ex.TraceID[:])
		labels[spanIdLabelName] = hex.EncodeToString(ex.SpanID[:])

		for _, attr := range ex.FilteredAttributes {
			labels[string(attr.Key)] = attr.Value.Emit()
		}

		result[i] = prompb.Exemplar{
			Value:     float64(ex.Value),
			Timestamp: ex.Time.UnixNano() / int64(time.Millisecond),
			Labels:    createLabelSet(labels),
		}
	}
	return result
}

// generateDataPointLabels returns a map of labels for a datapoint based on the metric name, labels, and attributes
func generateDataPointLabels(metricName string, labels map[string]string, attributes attribute.Set) map[string]string {
	result := addMetricName(metricName, labels)
	maps.Copy(result, generateAttributesLabels(attributes))
	return result
}

// createLabelSet combines attributes from a Record, resource, and extra attributes to create a
// slice of prompb.Label.
func createLabelSet(labels map[string]string) []prompb.Label {
	res := make([]prompb.Label, 0, len(labels))

	for l := range labels {
		res = append(res, prompb.Label{
			Name:  sanitize(l),
			Value: labels[l],
		})
	}

	return res
}

// Aggregation returns the default Aggregation to use for an instrument kind.
// Currently unused in this exporter, as it returns old sdk types. Therefore, in metric processing
// we directly inspects the metric data type.
// Retained for consistency with other OpenTelemetry exporters.
func (e *Exporter) Aggregation(k metric.InstrumentKind) metric.Aggregation {
	return metric.DefaultAggregationSelector(k)
}

// addHeaders adds required headers, an Authorization header, and all headers in the
// Config Headers map to a http request.
func (e *Exporter) addHeaders(req *http.Request) error {
	// Logz.io expects Snappy-compressed protobuf messages. These three headers are
	// hard-coded as they should be on every request.
	req.Header.Add("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", "logzio-go-sdk-metrics")

	// Add Authorization header
	bearerTokenString := "Bearer " + e.config.LogzioMetricsToken
	req.Header.Set("Authorization", bearerTokenString)

	return nil
}

// buildMessage creates a Snappy-compressed protobuf message from a slice of TimeSeries.
func (e *Exporter) buildMessage(timeseries []prompb.TimeSeries) ([]byte, error) {
	// Wrap the TimeSeries as a WriteRequest since Logz.io requires it.
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Convert the struct to a slice of bytes and then compress it.
	message := make([]byte, writeRequest.Size())
	written, err := writeRequest.MarshalToSizedBuffer(message)
	if err != nil {
		return nil, err
	}
	message = message[:written]
	compressed := snappy.Encode(nil, message)

	return compressed, nil
}

// buildRequest creates http POST request with a Snappy-compressed protocol buffer
// message as the body and with all the headers attached.
func (e *Exporter) buildRequest(message []byte) (*http.Request, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		e.config.LogzioMetricsListener,
		bytes.NewBuffer(message),
	)
	if err != nil {
		return nil, err
	}

	// Add the required headers and the headers from Config.Headers.
	err = e.addHeaders(req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// sendRequest sends http request using the Exporter's http Client.
func (e *Exporter) sendRequest(req *http.Request) error {
	// Set a client if there is no client.
	if e.config.client == nil {
		e.config.client = &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   e.config.RemoteTimeout,
		}
	}

	// Attempt to send request.
	res, err := e.config.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// The response should have a status code of 200.
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%v", res.Status)
	}
	return nil
}

// ForceFlush flushes any metric data held by an exporter.
func (e *Exporter) ForceFlush(ctx context.Context) error {
	// The exporter and client hold no state, nothing to flush.
	return ctx.Err()
}

// Shutdown flushes all metric data held by an exporter and releases any held computational resources.
func (e *Exporter) Shutdown(ctx context.Context) error {
	err := fmt.Errorf("HTTP exporter is shutdown")
	e.shutdownOnce.Do(func() {
		err = e.ForceFlush(ctx)

		if e.config.client != nil {
			e.clientMu.Lock()
			e.config.client.CloseIdleConnections()
			e.config.client = nil
			e.clientMu.Unlock()
		}
	})
	return err
}
