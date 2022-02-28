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
	"testing"

	metricsExporter "github.com/logzio/go-metrics-sdk"
	"github.com/stretchr/testify/require"
)

// TestValidate checks whether Validate() returns the correct error and sets the correct
// default values.
func TestValidate(t *testing.T) {
	tests := []struct {
		testName       string
		config         *metricsExporter.Config
		expectedConfig *metricsExporter.Config
		expectedError  error
	}{
		{
			testName:       "Config with Conflicting Bearer Tokens",
			config:         &exampleTwoBearerTokenConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrTwoBearerTokens,
		},
		{
			testName:       "Config with Conflicting Passwords",
			config:         &exampleTwoPasswordConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrTwoPasswords,
		},
		{
			testName:       "Config with no Password",
			config:         &exampleNoPasswordConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrNoBasicAuthPassword,
		},
		{
			testName:       "Config with no Username",
			config:         &exampleNoUsernameConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrNoBasicAuthUsername,
		},
		{
			testName:       "Config with Custom Timeout",
			config:         &exampleRemoteTimeoutConfig,
			expectedConfig: &validatedCustomTimeoutConfig,
			expectedError:  nil,
		},
		{
			testName:       "Config with no Endpoint",
			config:         &exampleNoEndpointConfig,
			expectedConfig: &validatedStandardConfig,
			expectedError:  nil,
		},
		{
			testName:       "Config with no Remote Timeout",
			config:         &exampleNoRemoteTimeoutConfig,
			expectedConfig: &validatedStandardConfig,
			expectedError:  nil,
		},
		{
			testName:       "Config with no Push Interval",
			config:         &exampleNoPushIntervalConfig,
			expectedConfig: &validatedStandardConfig,
			expectedError:  nil,
		},
		{
			testName:       "Config with no Client",
			config:         &exampleNoClientConfig,
			expectedConfig: &validatedStandardConfig,
			expectedError:  nil,
		},
		{
			testName:       "Config with both BasicAuth and BearerTokens",
			config:         &exampleTwoAuthConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrConflictingAuthorization,
		},
		{
			testName:       "Config with Invalid Quantiles",
			config:         &exampleInvalidQuantilesConfig,
			expectedConfig: nil,
			expectedError:  metricsExporter.ErrInvalidQuantiles,
		},
		{
			testName:       "Config with Valid Quantiles",
			config:         &exampleValidQuantilesConfig,
			expectedConfig: &validatedQuantilesConfig,
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			err := test.config.Validate()
			require.Equal(t, test.expectedError, err)
			if err == nil {
				require.Equal(t, test.config, test.expectedConfig)
			}
		})
	}
}
