// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package couchbasereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchbasereceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestNewFactory(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.EqualValues(t, typeStr, factory.Type())
			},
		},
		{
			desc: "creates a new factory with valid default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()

				var expectedCfg config.Receiver = &Config{
					ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
						ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
						CollectionInterval: 10 * time.Second,
					},
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: defaultEndpoint,
						Timeout:  10 * time.Second,
					},
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetricReceiver returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				_, err := factory.CreateMetricsReceiver(
					context.Background(),
					component.ReceiverCreateSettings{},
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
