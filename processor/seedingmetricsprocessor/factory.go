package seedingmetricsprocessor

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "seedingmetrics"
)

// NewFactory creates a factory for the redaction processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithMetrics(createMetricsProcessor),
	)
}

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	}
}

// createMetricsProcessor creates an instance of redaction for processing traces
func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Metrics,
) (component.MetricsProcessor, error) {
	oCfg := cfg.(*Config)

	processor, _ := newSeedingMetricsProcessor(ctx, oCfg, params.Logger, next)

	return processorhelper.NewMetricsProcessor(
		cfg,
		next,
		processor.processMetrics,
		processorhelper.WithCapabilities(processor.Capabilities()),
		processorhelper.WithStart(processor.Start),
		processorhelper.WithShutdown(processor.ShutDown))
}
