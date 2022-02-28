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
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type seedingMetricsProcessor struct {
	logger         *zap.Logger
	context        context.Context
	config         *Config
	nextConsumer   consumer.Metrics
	dbClient       *pebble.DB
	targetAppIdsMap map[string]bool
	ignoredAppIdsMap map[string]bool
}

func newSeedingMetricsProcessor(ctx context.Context, cfg *Config, logger *zap.Logger, next consumer.Metrics) (*seedingMetricsProcessor, error) {
	processor := &seedingMetricsProcessor{
		logger:       logger,
		context:      ctx,
		config:       cfg,
		nextConsumer: next,
	}

	return processor, nil
}

// Start is invoked during service startup.
func (processor *seedingMetricsProcessor) Start(context.Context, component.Host) error {
	dbFileName := processor.config.DbFileName
	// Env vars defined in service descriptor of XIS
	filePath := fmt.Sprintf("%s/%s", os.Getenv("SERVICE_VOLUME_PATH"), dbFileName)
	fmt.Printf("PeppleDB file path is %s\n", filePath)

	db, err := pebble.Open(filePath, &pebble.Options{})
	if err != nil {
		processor.logger.Fatal("error opening the database")
		return err
	}

	processor.dbClient = db

	// Create map for target and ignored appIDs
	processor.targetAppIdsMap = map[string]bool{}
	processor.ignoredAppIdsMap = map[string]bool{}

	for i := 0; i < len(processor.config.TargetAppIds); i++ {
		processor.targetAppIdsMap[processor.config.TargetAppIds[i]] = true
	}

	for j := 0; j < len(processor.config.IgnoredAppIds); j++ {
		processor.ignoredAppIdsMap[processor.config.IgnoredAppIds[j]] = true
	}

	return nil
}

func (processor *seedingMetricsProcessor) ShutDown(context.Context) error {
	return processor.dbClient.Close()
}

func (processor *seedingMetricsProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	startTime := time.Now()

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics()
		for j := 0; j < resourceMetrics.Len(); j++ {
			ilms := resourceMetrics.At(j).InstrumentationLibraryMetrics()
			for k := 0; k < ilms.Len(); k++ {
				ilm := ilms.At(k)
				metrics := ilm.Metrics()

				metrics.RemoveIf(func(metric pdata.Metric) bool {
					if metric.DataType() == pdata.MetricDataTypeSum {
						dps := metric.Sum().DataPoints()

						processor.logger.Debug(fmt.Sprintf("DPS length before RemoveIf: %d\n", dps.Len()))
						dps.RemoveIf(func(dp pdata.NumberDataPoint) bool {
							appId, ok := dp.Attributes().Get("appId")
							if !ok || !processor.shouldIncludeAppId(appId.AsString()) {
								processor.logger.Debug(fmt.Sprintf("Dropping metric for appID :%s\n", appId.AsString()))
								return true
							}

							notation, notationHash := processor.buildMetricNotation(metric.Name(), dp)

							// Only append zero value datapoint if notation combination appears for the first time
							if processor.isFirstInstanceOfMetricNotation(notationHash) {
								processor.logger.Debug(fmt.Sprintf("Storing notation :%s: with hash :%s: in seen map\n", notation, notationHash))

								err := processor.dbClient.Set([]byte(notationHash), []byte("1"), pebble.NoSync)
								if err != nil {
									processor.logger.Error("error writing key to dbClient",
										zap.String("key", notationHash), zap.Error(err))
								}
								if processor.config.ZeroInitialisation {
									dp.SetDoubleVal(float64(0))
									dp.SetIntVal(0)
								}
							}
							return false
						})

						processor.logger.Debug(fmt.Sprintf("DPS length after RemoveIf: %d\n", dps.Len()))
						return dps.Len() == 0
					}

					return true
				})
			}
		}
	}

	elapsed := time.Since(startTime)
	processor.logger.Debug(fmt.Sprintf("Processing metrics took %s\n", elapsed))

	return md, nil
}

func (processor *seedingMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (processor *seedingMetricsProcessor) shouldIncludeAppId(appId string) bool {
	if len(processor.targetAppIdsMap) > 0 {
		return processor.targetAppIdsMap[appId]
	}

	if len(processor.ignoredAppIdsMap) > 0 {
		return !processor.ignoredAppIdsMap[appId]
	}

	return true
}

// We can make it configurable in the future, if required.
func (processor *seedingMetricsProcessor) buildMetricNotation(metricName string, dp pdata.NumberDataPoint) (string, string) {
	var keys, pairs []string

	pairs = append(pairs, "metricName="+metricName)
	rawAttributes := dp.Attributes().AsRaw()

	for key := range rawAttributes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", key, rawAttributes[key]))
	}

	result := strings.Join(pairs, "|")

	sum256 := sha256.Sum256([]byte(result))
	return result, fmt.Sprintf("%x", sum256)
}

func (processor *seedingMetricsProcessor) isFirstInstanceOfMetricNotation(notationHash string) bool {
	_, closer, dbClientGetErr := processor.dbClient.Get([]byte(notationHash))
	if dbClientGetErr == pebble.ErrNotFound {
		processor.logger.Debug("Key not found in cache.", zap.String("key", notationHash))
		return true
	}

	if err := closer.Close(); err != nil {
		processor.logger.Error("Error closing the reader", zap.Error(err))
	}

	processor.logger.Debug("Successful cache hit.", zap.String("key", notationHash))
	return false
}

func toMb(mem uint64) uint64 { return mem / 1024 / 1024 }

func PrintMemUsage(before, after runtime.MemStats) {
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("\tTotalAlloc = %v MiB", toMb(after.TotalAlloc-before.TotalAlloc))
	fmt.Printf("\tHeapAlloc = %v MiB", toMb(after.HeapAlloc-before.HeapAlloc))
	fmt.Printf("\tSys = %v MiB", toMb(after.Sys-before.Sys))
	fmt.Printf("\tGc Cycles Run between = %v\n", after.NumGC-before.NumGC)
}
