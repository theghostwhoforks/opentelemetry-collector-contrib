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

package seedingmetricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/seedingmetricsprocessor"

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"time"
)

type seedingMetricsProcessor struct {
	logger       *zap.Logger
	context      context.Context
	config       *Config
	nextConsumer consumer.Metrics
}

func newSeedingMetricsProcessor(ctx context.Context, cfg *Config, logger *zap.Logger, next consumer.Metrics) (*seedingMetricsProcessor, error) {
	return &seedingMetricsProcessor{
		logger:       logger,
		context:      ctx,
		config:       cfg,
		nextConsumer: next,
	}, nil
}

// Start is invoked during service startup.
func (processor *seedingMetricsProcessor) Start(context.Context, component.Host) error {
	return nil
}

func (processor *seedingMetricsProcessor) ShutDown(context.Context) error {
	return nil
}

func (processor *seedingMetricsProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	fmt.Printf("Metric Count: %d, Datapoint count: %d\n", md.MetricCount(), md.DataPointCount())
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics()
		for j := 0; j < resourceMetrics.Len(); j++ {
			ilms := resourceMetrics.At(j).InstrumentationLibraryMetrics()
			for k := 0; k < ilms.Len(); k++ {
				ilm := ilms.At(k)
				fmt.Printf("Name: %s", ilm.InstrumentationLibrary().Name())
				metrics := ilm.Metrics()
				for l := 0; l < metrics.Len(); l++ {
					metric := metrics.At(l)
					fmt.Printf("Metric#%d DataType:%s, len: %d\n",
						l,
						metric.DataType(), metrics.Len())
					if metric.DataType() == pdata.MetricDataTypeSum {
						dps := metric.Sum().DataPoints()
						for m := 0; m < dps.Len(); m++ {
							dp := dps.At(m)
							fmt.Printf("MetricDataPoints: Value: %d, Attributes: %+v, StartTimestamp: %s, Timestamp: %s\n",
								dp.IntVal(),
								dp.Attributes().AsRaw(),
								dp.StartTimestamp().AsTime().Format(time.UnixDate),
								dp.Timestamp().AsTime().Format(time.UnixDate))
						}
					}
				}
			}
		}
	}
	return md, nil
}

func (processor *seedingMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
