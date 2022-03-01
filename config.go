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
	"fmt"
	"net/http"
	"time"
)

var (
	// ErrNoLogzioMetricsToken occurs when no Logz.io metrics token was provided for authorization.
	ErrNoLogzioMetricsToken = fmt.Errorf("Logz.io metrics token must not be empty")
)

// Config contains properties the Exporter uses to export metrics data to Logz.io.
type Config struct {
	LogzioMetricsListener string
	LogzioMetricsToken    string
	RemoteTimeout         time.Duration
	PushInterval          time.Duration
	HistogramBoundaries   []float64

	client                *http.Client
}

// Validate checks a Config struct for missing required properties and property conflicts.
// Additionally, it adds default values to missing properties when there is a default.
func (c *Config) Validate() error {
	// Check for valid Logz.io metrics token configuration.
	if c.LogzioMetricsToken == "" {
		return ErrNoLogzioMetricsToken
	}

	// Add default values for missing properties.
	if c.LogzioMetricsListener == "" {
		c.LogzioMetricsListener = "https://listener.logz.io:8053"
	}
	if c.RemoteTimeout == 0 {
		c.RemoteTimeout = 30 * time.Second
	}
	// Default time interval between pushes for the push controller is 10s.
	if c.PushInterval == 0 {
		c.PushInterval = 10 * time.Second
	}

	return nil
}
