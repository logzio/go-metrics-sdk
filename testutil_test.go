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

package metrics_exporter

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

func getResource() *resource.Resource {
	return resource.NewSchemaless(attribute.Key("service.name").String("test"))
}

func getScope() instrumentation.Scope {
	return instrumentation.Scope{
		Name:       "test-meter",
		Version:    "0.0.1",
		SchemaURL:  "",
		Attributes: attribute.Set{},
	}
}

// getSumMetric returns a resource metric with a sum aggregation record
func getSumMetric(value int64) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource: getResource(),
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: getScope(),
				Metrics: []metricdata.Metrics{
					{
						Name: "metric_sum",
						Data: metricdata.Sum[int64]{
							DataPoints: []metricdata.DataPoint[int64]{
								{
									Attributes: attribute.Set{},
									Time:       time.Now(),
									Value:      value,
								},
							},
							IsMonotonic: true,
						},
					},
				},
			},
		},
	}
}

// getGaugeMetric returns a resource metric with a gauge aggregation record
func getGaugeMetric(value int64) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource: getResource(),
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: getScope(),
				Metrics: []metricdata.Metrics{
					{
						Name: "metric_gauge",
						Data: metricdata.Gauge[int64]{
							DataPoints: []metricdata.DataPoint[int64]{
								{
									Attributes: attribute.Set{},
									Time:       time.Now(),
									Value:      value,
								},
							},
						},
					},
				},
			},
		},
	}
}

// getHistogramMetric returns a checkpoint set with a histogram aggregation record
func getHistogramMetric(count uint64, max, min metricdata.Extrema[int64], sum int64) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource: getResource(),
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: getScope(),
				Metrics: []metricdata.Metrics{
					{
						Name: "metric_histogram",
						Data: metricdata.Histogram[int64]{
							DataPoints: []metricdata.HistogramDataPoint[int64]{
								{
									Attributes:   attribute.Set{},
									Time:         time.Now(),
									Bounds:       []float64{0, 5},
									BucketCounts: []uint64{0, 1},
									Count:        count,
									Max:          max,
									Min:          min,
									Sum:          sum,
								},
							},
						},
					},
				},
			},
		},
	}
}

// The following variables hold expected TimeSeries values to be used in
// ConvertToTimeSeries tests.
var wantSumTimeSeries = []*prompb.TimeSeries{
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_sum",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 5,
			// Timestamp: this test verifies real timestamps
		}},
	},
}

var wantGaugeTimeSeries = []*prompb.TimeSeries{
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_gauge",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 5,
			// Timestamp: this test verifies real timestamps
		}},
	},
}

var wantHistogramTimeSeries = []*prompb.TimeSeries{
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram_max",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 2,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram_min",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 2,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram_sum",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 2,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram_count",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
		},
		Samples: []prompb.Sample{{
			Value: 1,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
			{
				Name:  "le",
				Value: "0",
			},
		},
		Samples: []prompb.Sample{{
			Value: 0,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
			{
				Name:  "le",
				Value: "5",
			},
		},
		Samples: []prompb.Sample{{
			Value: 1,
			// Timestamp: this test verifies real timestamps
		}},
	},
	{
		Labels: []prompb.Label{
			{
				Name:  "service_name",
				Value: "test",
			},
			{
				Name:  "__name__",
				Value: "metric_histogram",
			},
			{
				Name:  "otel_scope_name",
				Value: "test-meter",
			},
			{
				Name:  "otel_scope_version",
				Value: "0.0.1",
			},
			{
				Name:  "le",
				Value: "+inf",
			},
		},
		Samples: []prompb.Sample{{
			Value: 1,
			// Timestamp: this test verifies real timestamps
		}},
	},
}

func toMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
