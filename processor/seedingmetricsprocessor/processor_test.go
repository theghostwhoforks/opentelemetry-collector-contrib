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
