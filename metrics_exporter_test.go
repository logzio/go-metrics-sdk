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
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ValidConfig is a Config struct that should cause no errors.
var validConfig = Config{
	LogzioMetricsListener: "https://listener.logz.io:8053",
	LogzioMetricsToken:    "123456789a",
	RemoteTimeout:         30 * time.Second,
	PushInterval:          10 * time.Second,
	Quantiles:             []float64{0, 0.25, 0.5, 0.75, 1},
	AddMetricSuffixes:     true,
	ExternalLabels: map[string]string{
		"label": "value",
	},
	client: http.DefaultClient,
}

func TestTemporality(t *testing.T) {
	exporter := Exporter{}
	got := exporter.Temporality(metric.InstrumentKindCounter)
	want := metricdata.CumulativeTemporality

	if got != want {
		t.Errorf("Temporality() =  %q, want %q", got, want)
	}
}

func TestConvertToTimeSeries(t *testing.T) {
	// Setup exporter with default quantiles and histogram buckets
	exporter := Exporter{
		config: Config{},
	}

	startTime := time.Now()
	// Test conversions based on aggregation type
	tests := []struct {
		name       string
		input      *metricdata.ResourceMetrics
		want       []*prompb.TimeSeries
		wantLength int
	}{
		{
			name:       "convertFromSum",
			input:      getSumMetric(5),
			want:       wantSumTimeSeries,
			wantLength: 1,
		},
		{
			name:       "convertFromGauge",
			input:      getGaugeMetric(5),
			want:       wantGaugeTimeSeries,
			wantLength: 1,
		},
		{
			name:       "convertFromHistogram",
			input:      getHistogramMetric(1, metricdata.NewExtrema[int64](2), metricdata.NewExtrema[int64](2), 2),
			want:       wantHistogramTimeSeries,
			wantLength: 7,
		},
	}

	endTime := time.Now()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exporter.ConvertToTimeSeries(tt.input)
			want := tt.want

			// Check for errors and for the correct number of timeseries.
			assert.Nil(t, err, "ConvertToTimeSeries error")
			assert.Len(t, got, tt.wantLength, "Incorrect number of timeseries")

			// The TimeSeries cannot be compared easily using assert.ElementsMatch or
			// cmp.Equal since both the ordering of the timeseries and the ordering of the
			// attributes inside each timeseries can change. To get around this, all the
			// attributes and samples are added to maps first. There aren't many attributes or
			// samples, so this nested loop shouldn't be a bottleneck.
			gotAttributes := make(map[string]bool)
			wantAttributes := make(map[string]bool)
			gotSamples := make(map[string]bool)
			wantSamples := make(map[string]bool)

			for i := 0; i < len(got); i++ {
				for _, attr := range got[i].Labels {
					gotAttributes[attr.String()] = true
				}
				for _, attr := range want[i].Labels {
					wantAttributes[attr.String()] = true
				}
				for _, sample := range got[i].Samples {
					gotSamples[fmt.Sprint(sample.Value)] = true

					assert.LessOrEqual(t, toMillis(startTime), sample.Timestamp)
					assert.GreaterOrEqual(t, toMillis(endTime), sample.Timestamp)
				}
				for _, sample := range want[i].Samples {
					wantSamples[fmt.Sprint(sample.Value)] = true
				}
			}
			assert.Equal(t, wantAttributes, gotAttributes)
			assert.Equal(t, wantSamples, gotSamples)
		})
	}
}

// TestNewRawExporter tests whether NewRawExporter successfully creates an Exporter with
// the same Config struct as the one passed in.
func TestNew(t *testing.T) {
	exporter, err := New(validConfig)
	if err != nil {
		t.Fatalf("Failed to create exporter with error %v", err)
	}

	if !cmp.Equal(validConfig.LogzioMetricsListener, exporter.config.LogzioMetricsListener) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.LogzioMetricsToken, exporter.config.LogzioMetricsToken) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.RemoteTimeout, exporter.config.RemoteTimeout) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.PushInterval, exporter.config.PushInterval) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.Quantiles, exporter.config.Quantiles) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.HistogramBoundaries, exporter.config.HistogramBoundaries) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
	if !cmp.Equal(validConfig.client, exporter.config.client) {
		t.Fatalf("Got configuration %v, wanted %v", exporter.config, validConfig)
	}
}

// TestBuildMessage tests whether BuildMessage successfully returns a Snappy-compressed
// protobuf message.
func TestBuildMessage(t *testing.T) {
	exporter := Exporter{config: validConfig}
	timeseries := []prompb.TimeSeries{}

	// buildMessage returns the error that proto.Marshal() returns. Since the proto
	// package has its own tests, buildMessage should work as expected as long as there
	// are no errors.
	_, err := exporter.buildMessage(timeseries)
	require.NoError(t, err)
}

// TestBuildRequest tests whether a http request is a POST request, has the correct body,
// and has the correct headers.
func TestBuildRequest(t *testing.T) {
	// Make fake exporter and message for testing.
	var testMessage = []byte(`Test Message`)
	exporter := Exporter{config: validConfig}

	// Create the http request.
	req, err := exporter.buildRequest(testMessage)
	require.NoError(t, err)

	// Verify the http method, url, and body.
	require.Equal(t, req.Method, http.MethodPost)
	require.Equal(t, req.URL.String(), validConfig.LogzioMetricsListener)

	reqMessage, err := ioutil.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, reqMessage, testMessage)

	// Verify headers.
	require.Equal(t, req.Header.Get("Content-Encoding"), "snappy")
	require.Equal(t, req.Header.Get("Content-Type"), "application/x-protobuf")
	require.Equal(t, req.Header.Get("X-Prometheus-Remote-Write-Version"), "0.1.0")
}

// verifyExporterRequest checks a HTTP request from the export pipeline. It checks whether
// the request contains a correctly formatted remote_write body and the required headers.
func verifyExporterRequest(req *http.Request) error {
	// Check for required headers.
	if req.Header.Get("X-Prometheus-Remote-Write-Version") != "0.1.0" ||
		req.Header.Get("Content-Encoding") != "snappy" ||
		req.Header.Get("Content-Type") != "application/x-protobuf" {
		return fmt.Errorf("Request does not contain the three required headers")
	}

	// Check whether request body is in the correct format.
	compressed, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("Failed to read request body")
	}
	uncompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		return fmt.Errorf("Failed to uncompress request body")
	}
	wr := &prompb.WriteRequest{}
	err = wr.Unmarshal(uncompressed)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal message into WriteRequest struct")
	}

	// Check whether the request contains the correct data.
	expectedWriteRequest := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Samples: []prompb.Sample{
					{
						Value:     float64(123),
						Timestamp: int64(time.Nanosecond) * time.Time{}.UnixNano() / int64(time.Millisecond),
					},
				},
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "test_name",
					},
				},
			},
		},
	}
	if !cmp.Equal(wr, expectedWriteRequest) {
		return fmt.Errorf("request does not contain the expected contents")
	}

	return nil
}

// TestSendRequest checks if the Exporter can successfully send a http request with a
// correctly formatted body and the correct headers. A test server returns different
// status codes to test if the Exporter responds to a send failure correctly.
func TestSendRequest(t *testing.T) {
	tests := []struct {
		testName         string
		config           *Config
		expectedError    error
		isStatusNotFound bool
	}{
		{
			testName:         "Successful Export",
			config:           &validConfig,
			expectedError:    nil,
			isStatusNotFound: false,
		},
	}

	// Set up a test server to receive the request. The server responds with a 400 Bad
	// Request status code if any headers are missing or if the body does not have the
	// correct contents. Additionally, the server can respond with status code 404 Not
	// Found to simulate send failures.
	handler := func(rw http.ResponseWriter, req *http.Request) {
		err := verifyExporterRequest(req)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return a status code 400 if header isStatusNotFound is "true". Otherwise,
		// return status code 200.
		if req.Header.Get("isStatusNotFound") == "true" {
			rw.WriteHeader(http.StatusNotFound)
		} else {
			rw.WriteHeader(http.StatusOK)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			// Set up an Exporter that uses the test server's endpoint and attaches the
			// test's isStatusNotFound header.
			test.config.LogzioMetricsListener = server.URL
			exporter := Exporter{config: *test.config}

			// Create a test TimeSeries struct.
			timeSeries := []prompb.TimeSeries{
				{
					Samples: []prompb.Sample{
						{
							Value:     float64(123),
							Timestamp: int64(time.Nanosecond) * time.Time{}.UnixNano() / int64(time.Millisecond),
						},
					},
					Labels: []prompb.Label{
						{
							Name:  "__name__",
							Value: "test_name",
						},
					},
				},
			}

			// Create a Snappy-compressed message.
			msg, err := exporter.buildMessage(timeSeries)
			require.NoError(t, err)

			// Create a http POST request with the compressed message.
			req, err := exporter.buildRequest(msg)
			require.NoError(t, err)

			// Send the request to the test server and verify the error.
			err = exporter.sendRequest(req)
			if err != nil {
				errorString := err.Error()
				require.Equal(t, errorString, test.expectedError.Error())
			} else {
				require.NoError(t, test.expectedError)
			}
		})
	}
}
