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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type metricNotation struct {
	metricName    string
	appId         string
	environmentId string
	instanceId    string
	userTier      string
}

type seedingMetricsProcessor struct {
	logger         *zap.Logger
	context        context.Context
	config         *Config
	nextConsumer   consumer.Metrics
	sampledMetrics map[metricNotation]bool
}

func newSeedingMetricsProcessor(ctx context.Context, cfg *Config, logger *zap.Logger, next consumer.Metrics) (*seedingMetricsProcessor, error) {
	processor := &seedingMetricsProcessor{
		logger:       logger,
		context:      ctx,
		config:       cfg,
		nextConsumer: next,
	}

	processor.sampledMetrics = make(map[metricNotation]bool)

	return processor, nil
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
				metrics := ilm.Metrics()
				fmt.Printf("Name: %s, len: %d", ilm.InstrumentationLibrary().Name(), metrics.Len())
				for l := 0; l < metrics.Len(); l++ {
					metric := metrics.At(l)
					fmt.Printf("Metric#%d Name:%s, DataType:%s\n",
						l,
						metric.Name(),
						metric.DataType())
					if metric.DataType() == pdata.MetricDataTypeSum {
						dps := metric.Sum().DataPoints()
						if dps.Len() < 1 {
							continue
						}

						for m := 0; m < dps.Len(); m++ {
							dp := dps.At(m)
							notation := processor.buildMetricNotation(metric.Name(), dp)

							// Only append zero value datapoint if notation combination appears for the first time
							if processor.isFirstInstanceOfMetricNotation(notation) {
								processor.sampledMetrics[notation] = true
								dp.SetDoubleVal(float64(0))
								dp.SetIntVal(0)
							}
						}

						for m := 0; m < dps.Len(); m++ {
							dp := dps.At(m)
							fmt.Printf("MetricDataPoints: Value: %d:%f, Attributes: %+v, StartTimestamp: %s, Timestamp: %s\n",
								dp.IntVal(),
								dp.DoubleVal(),
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

// This check logic only work for certain attribtues struct for now.
// We can make it configurable in the future.
func (processor *seedingMetricsProcessor) buildMetricNotation(metricName string, dp pdata.NumberDataPoint) metricNotation {
	rawAttributes := dp.Attributes().AsRaw()

	appId, found := rawAttributes["appId"].(string)
	if !found {
		appId = ""
	}

	environmentId, found := rawAttributes["environmentId"].(string)
	if !found {
		environmentId = ""
	}

	instanceId, found := rawAttributes["instanceId"].(string)
	if !found {
		instanceId = ""
	}

	userTier, found := rawAttributes["userTier"].(string)
	if !found {
		userTier = ""
	}

	return metricNotation{
		metricName:    metricName,
		appId:         appId,
		environmentId: environmentId,
		instanceId:    instanceId,
		userTier:      userTier}
}

func (processor *seedingMetricsProcessor) isFirstInstanceOfMetricNotation(notation metricNotation) bool {
	return !processor.sampledMetrics[notation]
}
