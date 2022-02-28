// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seedingmetricsprocessor

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration settings for the seedingmetrics processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	DbFileName string `mapstructure:"db_file_name"`

	TargetAppIds []string `mapstructure:"target_app_ids"`

	IgnoredAppIds []string `mapstructure:"ignored_app_ids"`

	ZeroInitialisation bool `mapstructure:"zero_initialisation"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.DbFileName) == 0 {
		return fmt.Errorf(
			"invalid attribute to read the route's value from: %w",
			errors.New("the db_file_name property is empty"),
		)
	}
	return nil
}
