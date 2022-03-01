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

package metrics_exporter_test

import (
	"time"

	metricsExporter "github.com/logzio/go-metrics-sdk"
)

// Config struct with default values. This is used to verify the output of Validate().
var validatedStandardConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
	Quantiles: []float64{0.5, 0.9, 0.95, 0.99},
}

// Config struct with default values other than the remote timeout. This is used to verify
// the output of Validate().
var validatedCustomTimeoutConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 10 * time.Second,
	PushInterval:  10 * time.Second,
	Quantiles: []float64{0.5, 0.9, 0.95, 0.99},
}

// Config struct with default values other than the quantiles. This is used to verify
// the output of Validate().
var validatedQuantilesConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
	Quantiles:     []float64{0, 0.5, 1},
}

// Example Config struct with a custom remote timeout.
var exampleRemoteTimeoutConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	PushInterval:  10 * time.Second,
	RemoteTimeout: 10 * time.Second,
}

// Example Config struct without a remote timeout.
var exampleNoRemoteTimeoutConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	PushInterval: 10 * time.Second,
}

// Example Config struct without a push interval.
var exampleNoPushIntervalConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
}

// Example Config struct without a logzio metrics listener.
var exampleNoLogzioMetricsListenerConfig = metricsExporter.Config{
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
}

// Example Config struct without a logzio metrics token.
var exampleNoLogzioMetricsTokenConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
}

// Example Config struct with invalid quantiles.
var exampleInvalidQuantilesConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
	Quantiles:     []float64{0, 1, 2, 3},
}

// Example Config struct with valid quantiles.
var exampleValidQuantilesConfig = metricsExporter.Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken: "123456789a",
	RemoteTimeout: 30 * time.Second,
	PushInterval:  10 * time.Second,
	Quantiles:     []float64{0, 0.5, 1},
}
