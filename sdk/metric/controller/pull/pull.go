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

package pull // import "github.com/Ch1f/otel/sdk/metric/controller/pull"

import (
	"context"
	"time"

	"github.com/Ch1f/otel/api/metric"
	"github.com/Ch1f/otel/api/metric/registry"
	export "github.com/Ch1f/otel/sdk/export/metric"
	sdk "github.com/Ch1f/otel/sdk/metric"
	controllerTime "github.com/Ch1f/otel/sdk/metric/controller/time"
	processor "github.com/Ch1f/otel/sdk/metric/processor/basic"
	"github.com/Ch1f/otel/sdk/resource"
)

// DefaultCachePeriod determines how long a recently-computed result
// will be returned without gathering metric data again.
const DefaultCachePeriod time.Duration = 10 * time.Second

// Controller manages access to a *sdk.Accumulator and
// *basic.Processor.  Use Provider() for obtaining Meters.  Use
// Foreach() for accessing current records.
type Controller struct {
	accumulator *sdk.Accumulator
	processor   *processor.Processor
	provider    *registry.Provider
	period      time.Duration
	lastCollect time.Time
	clock       controllerTime.Clock
	checkpoint  export.CheckpointSet
}

// New returns a *Controller configured with an aggregation selector and options.
func New(aselector export.AggregatorSelector, eselector export.ExportKindSelector, options ...Option) *Controller {
	config := &Config{
		Resource:    resource.Empty(),
		CachePeriod: DefaultCachePeriod,
	}
	for _, opt := range options {
		opt.Apply(config)
	}
	// This controller uses WithMemory() as a requirement to
	// support multiple readers.
	processor := processor.New(aselector, eselector, processor.WithMemory(true))
	accum := sdk.NewAccumulator(
		processor,
		sdk.WithResource(config.Resource),
	)
	return &Controller{
		accumulator: accum,
		processor:   processor,
		provider:    registry.NewProvider(accum),
		period:      config.CachePeriod,
		checkpoint:  processor.CheckpointSet(),
		clock:       controllerTime.RealClock{},
	}
}

// SetClock sets the clock used for caching.  For testing purposes.
func (c *Controller) SetClock(clock controllerTime.Clock) {
	c.processor.Lock()
	defer c.processor.Unlock()
	c.clock = clock
}

// Provider returns a metric.Provider for the implementation managed
// by this controller.
func (c *Controller) Provider() metric.Provider {
	return c.provider
}

// Foreach gives the caller read-locked access to the current
// export.CheckpointSet.
func (c *Controller) ForEach(ks export.ExportKindSelector, f func(export.Record) error) error {
	c.processor.RLock()
	defer c.processor.RUnlock()

	return c.checkpoint.ForEach(ks, f)
}

// Collect requests a collection.  The collection will be skipped if
// the last collection is aged less than the CachePeriod.
func (c *Controller) Collect(ctx context.Context) error {
	c.processor.Lock()
	defer c.processor.Unlock()

	if c.period > 0 {
		now := c.clock.Now()
		elapsed := now.Sub(c.lastCollect)

		if elapsed < c.period {
			return nil
		}
		c.lastCollect = now
	}

	c.processor.StartCollection()
	c.accumulator.Collect(ctx)
	err := c.processor.FinishCollection()
	c.checkpoint = c.processor.CheckpointSet()
	return err
}
