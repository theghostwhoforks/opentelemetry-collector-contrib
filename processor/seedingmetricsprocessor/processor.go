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
	"time"

	"github.com/gomodule/redigo/redis"
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
	redisPool      *redis.Pool
}

func newSeedingMetricsProcessor(ctx context.Context, cfg *Config, logger *zap.Logger, next consumer.Metrics) (*seedingMetricsProcessor, error) {
	processor := &seedingMetricsProcessor{
		logger:       logger,
		context:      ctx,
		config:       cfg,
		nextConsumer: next,
	}

	processor.redisPool = newPool()

	return processor, nil
}

// Start is invoked during service startup.
func (processor *seedingMetricsProcessor) Start(context.Context, component.Host) error {
	fmt.Println("Starting seeding metrics processor")
	return nil
}

func (processor *seedingMetricsProcessor) ShutDown(context.Context) error {
	fmt.Println("Shuting down seeding metrics processor")
	return nil
}

func (processor *seedingMetricsProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	var before, after runtime.MemStats
	start := time.Now()
	runtime.ReadMemStats(&before)

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
						conn := processor.redisPool.Get()
						defer conn.Close()
						dps := metric.Sum().DataPoints()
						if dps.Len() < 1 {
							continue
						}

						for m := 0; m < dps.Len(); m++ {
							dp := dps.At(m)

							notation, notationHash := processor.buildMetricNotation(metric.Name(), dp)

							// Only append zero value datapoint if notation combination appears for the first time
							_, err := redis.String(conn.Do("GET", notationHash))
							if err != nil && err != redis.ErrNil {
								fmt.Printf("error checking key existence %v", err)
							}
							if err == nil {
								fmt.Printf("Cache hit for hashkey %s\n", notationHash)
							}
							if err == redis.ErrNil {
								fmt.Printf("Storing notation :%s: with hash :%s: into redis...\n", notation, notationHash)
								_, err := conn.Do("SET", notationHash, "")
								if err != nil {
									fmt.Printf("error setting key %s: %v", notationHash, err)
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

	elapsed := time.Since(start)
	fmt.Printf("Process metrics took about %s\n", elapsed)
	fmt.Printf("Memory Usage after processing metrics\n")
	runtime.ReadMemStats(&after)
	PrintMemUsage(before, after)
	return md, nil
}

func (processor *seedingMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// We can make it configurable in the future, if required.
func (processor *seedingMetricsProcessor) buildMetricNotation(metricName string, dp pdata.NumberDataPoint) (string, string) {
	var pairs []string
	result := "metricName=" + metricName
	rawAttributes := dp.Attributes().AsRaw()

	for key, val := range rawAttributes {
		pairs = append(pairs, key+"="+fmt.Sprintf("%v", val))
	}

	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i] < pairs[j] })

	for _, pair := range pairs {
		result += "|" + pair
	}

	sum256 := sha256.Sum256([]byte(result))
	return result, fmt.Sprintf("%x", sum256)
}

func toMb(mem uint64) uint64 { return mem / 1024 / 1024 }

func PrintMemUsage(before, after runtime.MemStats) {
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("\tTotalAlloc = %v MiB", toMb(after.TotalAlloc-before.TotalAlloc))
	fmt.Printf("\tHeapAlloc = %v MiB", toMb(after.HeapAlloc-before.HeapAlloc))
	fmt.Printf("\tSys = %v MiB", toMb(after.Sys-before.Sys))
	fmt.Printf("\tGc Cycles Run between = %v\n", after.NumGC-before.NumGC)
}

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,

		Dial: func() (redis.Conn, error) {
			redisHost := os.Getenv("REDISX_SIDECAR_MAP_HOST") + ":" + os.Getenv("REDISX_SIDECAR_MAP_PORT")
			fmt.Printf("Redis host is %s", redisHost)
			c, err := redis.Dial("tcp", redisHost)
			if err != nil {
					return nil, err
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
