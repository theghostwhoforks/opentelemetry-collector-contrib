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

//go:build integration
// +build integration

package memcachedreceiver

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/otlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
)

func TestIntegration(t *testing.T) {
	cs := container.New(t)
	c := cs.StartImage("memcached:1.6-alpine", container.WithPortReady(11211))

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = c.AddrForPort(11211)

	consumer := new(consumertest.MetricsSink)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	require.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 15*time.Second, 1*time.Second, "failed to receive at least 5 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	md := consumer.AllMetrics()[0]

	require.Equal(t, 1, md.ResourceMetrics().Len())

	ilms := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())

	aMetricSlice := ilms.At(0).Metrics()
	require.Equal(t, 11, aMetricSlice.Len())

	expectedFileBytes, err := ioutil.ReadFile("./testdata/expected_metrics/test_scraper/expected.json")
	require.NoError(t, err)

	unmarshaller := otlp.NewJSONMetricsUnmarshaler()
	expectedMetrics, err := unmarshaller.UnmarshalMetrics(expectedFileBytes)
	require.NoError(t, err)

	eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	require.NoError(t, scrapertest.CompareMetricSlices(eMetricSlice, aMetricSlice, scrapertest.IgnoreValues()))
}
