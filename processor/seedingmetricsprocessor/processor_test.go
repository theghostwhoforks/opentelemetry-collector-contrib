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

package seedingmetricsprocessor

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/seedingmetricsprocessor/testdata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"testing"
)

func TestSeedingMetricsProcessor_ProcessMetrics(t *testing.T) {
	next := new(consumertest.MetricsSink)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	ctx := context.Background()
	processor, err := factory.CreateMetricsProcessor(
		ctx, componenttest.NewNopProcessorCreateSettings(), cfg, next)

	assert.Nil(t, err)
	assert.NotNil(t, processor)

	metrics := testdata.GenerateMetricsManyMetricsSameResource(100)
	err = processor.ConsumeMetrics(ctx, metrics)

	assert.Nil(t, err)

	allMetrics := next.AllMetrics()[0]
	assert.True(t, allMetrics.MetricCount() == metrics.MetricCount(), "Same number of metrics were not propagated to the next consumer. Expected %d, got %d", metrics.MetricCount(), allMetrics.MetricCount())
}
