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
	seenMetricsMap map[string]struct{}
	dbClient       *pebble.DB
}

func newSeedingMetricsProcessor(ctx context.Context, cfg *Config, logger *zap.Logger, next consumer.Metrics) (*seedingMetricsProcessor, error) {
	processor := &seedingMetricsProcessor{
		logger:       logger,
		context:      ctx,
		config:       cfg,
		nextConsumer: next,
	}

	processor.seenMetricsMap = make(map[string]struct{})

	return processor, nil
}

// Start is invoked during service startup.
func (processor *seedingMetricsProcessor) Start(context.Context, component.Host) error {

	//dir, _ := ioutil.TempDir(".", "seen-metrics-processor-*")

	// Env vars defined in service descriptor of XIS
	filePath := os.Getenv("SERVICE_VOLUME_PATH") + "/" + os.Getenv("SIDECAR_MAP_FILE_NAME")
	fmt.Printf("PeppleDB file path is %s\n", filePath)

	db, err := pebble.Open(filePath, &pebble.Options{})
	if err != nil {
		processor.logger.Fatal("error opening the database")
		return err
	}

	processor.dbClient = db

	return nil
}

func (processor *seedingMetricsProcessor) ShutDown(context.Context) error {
	return processor.dbClient.Close()
}

func (processor *seedingMetricsProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	startTime := time.Now()

	//fmt.Printf("Metric Count: %d, Datapoint count: %d\n", md.MetricCount(), md.DataPointCount())
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

							notation, notationHash := processor.buildMetricNotation(metric.Name(), dp)

							// Only append zero value datapoint if notation combination appears for the first time
							if processor.isFirstInstanceOfMetricNotation(notationHash) {
								fmt.Printf("Storing notation :%s: with hash :%s: in seen map\n", notation, notationHash)
								processor.seenMetricsMap[notationHash] = struct{}{}

								err := processor.dbClient.Set([]byte(notationHash), []byte("1"), pebble.Sync)
								if err != nil {
									processor.logger.Error("error writing key to dbClient",
										zap.String("key", notationHash), zap.Error(err))
								}
								dp.SetDoubleVal(float64(0))
								dp.SetIntVal(0)
							}
						}
					}
				}
			}
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Processing metrics took %s", elapsed)

	processor.logger.Info("Memory Usage after processing metrics\n")
	runtime.ReadMemStats(&after)
	PrintMemUsage(before, after)
	return md, nil
}

func (processor *seedingMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// We can make it configurable in the future, if required.
func (processor *seedingMetricsProcessor) buildMetricNotation(metricName string, dp pdata.NumberDataPoint) (string, string) {
	var keys, pairs []string

	pairs = append(pairs, "metricName="+metricName)
	rawAttributes := dp.Attributes().AsRaw()

	for key, _ := range rawAttributes {
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
		processor.logger.Info("Key not found in cache.", zap.String("key", notationHash))
		return true
	}

	if err := closer.Close(); err != nil {
		processor.logger.Error("Error closing the reader", zap.Error(err))
	}

	processor.logger.Info("Successful cache hit.", zap.String("key", notationHash))
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
