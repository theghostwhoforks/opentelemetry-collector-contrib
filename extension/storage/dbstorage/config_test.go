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

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"
import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		errWanted error
	}{
		{
			"Missing driver name",
			Config{DataSource: "foo"},
			errors.New("missing driver name for /blah"),
		},
		{
			"Missing datasource",
			Config{DriverName: "foo"},
			errors.New("missing datasource for /blah"),
		},
		{
			"valid",
			Config{DriverName: "foo", DataSource: "bar"},
			nil,
		},
	}

	for _, test := range tests {
		test.config.SetIDName("blah")
		err := test.config.Validate()
		if test.errWanted == nil {
			assert.NoError(t, err)
		} else {
			assert.Equal(t, test.errWanted, err)
		}
	}
}
