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

//go:generate stringer -type=ExportKind

package metric // import "github.com/Ch1f/otel/sdk/export/metric"

import (
	"context"
	"sync"
	"time"

	"github.com/Ch1f/otel/api/label"
	"github.com/Ch1f/otel/api/metric"
	"github.com/Ch1f/otel/sdk/export/metric/aggregation"
	"github.com/Ch1f/otel/sdk/resource"
)

// Processor is responsible for deciding which kind of aggregation to
// use (via AggregatorSelector), gathering exported results from the
// SDK during collection, and deciding over which dimensions to group
// the exported data.
//
// The SDK supports binding only one of these interfaces, as it has
// the sole responsibility of determining which Aggregator to use for
// each record.
//
// The embedded AggregatorSelector interface is called (concurrently)
// in instrumentation context to select the appropriate Aggregator for
// an instrument.
//
// The `Process` method is called during collection in a
// single-threaded context from the SDK, after the aggregator is
// checkpointed, allowing the processor to build the set of metrics
// currently being exported.
type Processor interface {
	// AggregatorSelector is responsible for selecting the
	// concrete type of Aggregator used for a metric in the SDK.
	//
	// This may be a static decision based on fields of the
	// Descriptor, or it could use an external configuration
	// source to customize the treatment of each metric
	// instrument.
	//
	// The result from AggregatorSelector.AggregatorFor should be
	// the same type for a given Descriptor or else nil.  The same
	// type should be returned for a given descriptor, because
	// Aggregators only know how to Merge with their own type.  If
	// the result is nil, the metric instrument will be disabled.
	//
	// Note that the SDK only calls AggregatorFor when new records
	// require an Aggregator. This does not provide a way to
	// disable metrics with active records.
	AggregatorSelector

	// Process is called by the SDK once per internal record,
	// passing the export Accumulation (a Descriptor, the corresponding
	// Labels, and the checkpointed Aggregator).  This call has no
	// Context argument because it is expected to perform only
	// computation.  An SDK is not expected to call exporters from
	// with Process, use a controller for that (see
	// ./controllers/{pull,push}.
	Process(Accumulation) error
}

// AggregatorSelector supports selecting the kind of Aggregator to
// use at runtime for a specific metric instrument.
type AggregatorSelector interface {
	// AggregatorFor allocates a variable number of aggregators of
	// a kind suitable for the requested export.  This method
	// initializes a `...*Aggregator`, to support making a single
	// allocation.
	//
	// When the call returns without initializing the *Aggregator
	// to a non-nil value, the metric instrument is explicitly
	// disabled.
	//
	// This must return a consistent type to avoid confusion in
	// later stages of the metrics export process, i.e., when
	// Merging or Checkpointing aggregators for a specific
	// instrument.
	//
	// Note: This is context-free because the aggregator should
	// not relate to the incoming context.  This call should not
	// block.
	AggregatorFor(*metric.Descriptor, ...*Aggregator)
}

// Aggregator implements a specific aggregation behavior, e.g., a
// behavior to track a sequence of updates to an instrument.  Sum-only
// instruments commonly use a simple Sum aggregator, but for the
// distribution instruments (ValueRecorder, ValueObserver) there are a
// number of possible aggregators with different cost and accuracy
// tradeoffs.
//
// Note that any Aggregator may be attached to any instrument--this is
// the result of the OpenTelemetry API/SDK separation.  It is possible
// to attach a Sum aggregator to a ValueRecorder instrument or a
// MinMaxSumCount aggregator to a Counter instrument.
type Aggregator interface {
	// Aggregation returns an Aggregation interface to access the
	// current state of this Aggregator.  The caller is
	// responsible for synchronization and must not call any the
	// other methods in this interface concurrently while using
	// the Aggregation.
	Aggregation() aggregation.Aggregation

	// Update receives a new measured value and incorporates it
	// into the aggregation.  Update() calls may be called
	// concurrently.
	//
	// Descriptor.NumberKind() should be consulted to determine
	// whether the provided number is an int64 or float64.
	//
	// The Context argument comes from user-level code and could be
	// inspected for a `correlation.Map` or `trace.SpanContext`.
	Update(context.Context, metric.Number, *metric.Descriptor) error

	// SynchronizedMove is called during collection to finish one
	// period of aggregation by atomically saving the
	// currently-updating state into the argument Aggregator AND
	// resetting the current value to the zero state.
	//
	// SynchronizedMove() is called concurrently with Update().  These
	// two methods must be synchronized with respect to each
	// other, for correctness.
	//
	// After saving a synchronized copy, the Aggregator can be converted
	// into one or more of the interfaces in the `aggregation` sub-package,
	// according to kind of Aggregator that was selected.
	//
	// This method will return an InconsistentAggregatorError if
	// this Aggregator cannot be copied into the destination due
	// to an incompatible type.
	//
	// This call has no Context argument because it is expected to
	// perform only computation.
	SynchronizedMove(destination Aggregator, descriptor *metric.Descriptor) error

	// Merge combines the checkpointed state from the argument
	// Aggregator into this Aggregator.  Merge is not synchronized
	// with respect to Update or SynchronizedMove.
	//
	// The owner of an Aggregator being merged is responsible for
	// synchronization of both Aggregator states.
	Merge(Aggregator, *metric.Descriptor) error
}

// Subtractor is an optional interface implemented by some
// Aggregators.  An Aggregator must support `Subtract()` in order to
// be configured for a Precomputed-Sum instrument (SumObserver,
// UpDownSumObserver) using a DeltaExporter.
type Subtractor interface {
	// Subtract subtracts the `operand` from this Aggregator and
	// outputs the value in `result`.
	Subtract(operand, result Aggregator, descriptor *metric.Descriptor) error
}

// Exporter handles presentation of the checkpoint of aggregate
// metrics.  This is the final stage of a metrics export pipeline,
// where metric data are formatted for a specific system.
type Exporter interface {
	// Export is called immediately after completing a collection
	// pass in the SDK.
	//
	// The Context comes from the controller that initiated
	// collection.
	//
	// The CheckpointSet interface refers to the Processor that just
	// completed collection.
	Export(context.Context, CheckpointSet) error

	// ExportKindSelector is an interface used by the Processor
	// in deciding whether to compute Delta or Cumulative
	// Aggregations when passing Records to this Exporter.
	ExportKindSelector
}

// ExportKindSelector is a sub-interface of Exporter used to indicate
// whether the Processor should compute Delta or Cumulative
// Aggregations.
type ExportKindSelector interface {
	// ExportKindFor should return the correct ExportKind that
	// should be used when exporting data for the given metric
	// instrument and Aggregator kind.
	ExportKindFor(*metric.Descriptor, aggregation.Kind) ExportKind
}

// CheckpointSet allows a controller to access a complete checkpoint of
// aggregated metrics from the Processor.  This is passed to the
// Exporter which may then use ForEach to iterate over the collection
// of aggregated metrics.
type CheckpointSet interface {
	// ForEach iterates over aggregated checkpoints for all
	// metrics that were updated during the last collection
	// period. Each aggregated checkpoint returned by the
	// function parameter may return an error.
	//
	// The ExportKindSelector argument is used to determine
	// whether the Record is computed using Delta or Cumulative
	// aggregation.
	//
	// ForEach tolerates ErrNoData silently, as this is
	// expected from the Meter implementation. Any other kind
	// of error will immediately halt ForEach and return
	// the error to the caller.
	ForEach(ExportKindSelector, func(Record) error) error

	// Locker supports locking the checkpoint set.  Collection
	// into the checkpoint set cannot take place (in case of a
	// stateful processor) while it is locked.
	//
	// The Processor attached to the Accumulator MUST be called
	// with the lock held.
	sync.Locker

	// RLock acquires a read lock corresponding to this Locker.
	RLock()
	// RUnlock releases a read lock corresponding to this Locker.
	RUnlock()
}

// Metadata contains the common elements for exported metric data that
// are shared by the Accumulator->Processor and Processor->Exporter
// steps.
type Metadata struct {
	descriptor *metric.Descriptor
	labels     *label.Set
	resource   *resource.Resource
}

// Accumulation contains the exported data for a single metric instrument
// and label set, as prepared by an Accumulator for the Processor.
type Accumulation struct {
	Metadata
	aggregator Aggregator
}

// Record contains the exported data for a single metric instrument
// and label set, as prepared by the Processor for the Exporter.
// This includes the effective start and end time for the aggregation.
type Record struct {
	Metadata
	aggregation aggregation.Aggregation
	start       time.Time
	end         time.Time
}

// Descriptor describes the metric instrument being exported.
func (m Metadata) Descriptor() *metric.Descriptor {
	return m.descriptor
}

// Labels describes the labels associated with the instrument and the
// aggregated data.
func (m Metadata) Labels() *label.Set {
	return m.labels
}

// Resource contains common attributes that apply to this metric event.
func (m Metadata) Resource() *resource.Resource {
	return m.resource
}

// NewAccumulation allows Accumulator implementations to construct new
// Accumulations to send to Processors. The Descriptor, Labels, Resource,
// and Aggregator represent aggregate metric events received over a single
// collection period.
func NewAccumulation(descriptor *metric.Descriptor, labels *label.Set, resource *resource.Resource, aggregator Aggregator) Accumulation {
	return Accumulation{
		Metadata: Metadata{
			descriptor: descriptor,
			labels:     labels,
			resource:   resource,
		},
		aggregator: aggregator,
	}
}

// Aggregator returns the checkpointed aggregator. It is safe to
// access the checkpointed state without locking.
func (r Accumulation) Aggregator() Aggregator {
	return r.aggregator
}

// NewRecord allows Processor implementations to construct export
// records.  The Descriptor, Labels, and Aggregator represent
// aggregate metric events received over a single collection period.
func NewRecord(descriptor *metric.Descriptor, labels *label.Set, resource *resource.Resource, aggregation aggregation.Aggregation, start, end time.Time) Record {
	return Record{
		Metadata: Metadata{
			descriptor: descriptor,
			labels:     labels,
			resource:   resource,
		},
		aggregation: aggregation,
		start:       start,
		end:         end,
	}
}

// Aggregation returns the aggregation, an interface to the record and
// its aggregator, dependent on the kind of both the input and exporter.
func (r Record) Aggregation() aggregation.Aggregation {
	return r.aggregation
}

// StartTime is the start time of the interval covered by this aggregation.
func (r Record) StartTime() time.Time {
	return r.start
}

// EndTime is the end time of the interval covered by this aggregation.
func (r Record) EndTime() time.Time {
	return r.end
}

// ExportKind indicates the kind of data exported by an exporter.
// These bits may be OR-d together when multiple exporters are in use.
type ExportKind int

const (
	// CumulativeExporter indicates that the Exporter expects a
	// Cumulative Aggregation.
	CumulativeExporter ExportKind = 1 // e.g., Prometheus

	// DeltaExporter indicates that the Exporter expects a
	// Delta Aggregation.
	DeltaExporter ExportKind = 2 // e.g., StatsD

	// PassThroughExporter indicates that the Exporter expects
	// either a Cumulative or a Delta Aggregation, whichever does
	// not require maintaining state for the given instrument.
	PassThroughExporter ExportKind = 4 // e.g., OTLP
)

// Includes tests whether `kind` includes a specific kind of
// exporter.
func (kind ExportKind) Includes(has ExportKind) bool {
	return kind&has != 0
}

// ExportKindFor returns a constant, as an implementation of ExportKindSelector.
func (kind ExportKind) ExportKindFor(_ *metric.Descriptor, _ aggregation.Kind) ExportKind {
	return kind
}

// MemoryRequired returns whether an exporter of this kind requires
// memory to export correctly.
func (kind ExportKind) MemoryRequired(mkind metric.Kind) bool {
	switch mkind {
	case metric.ValueRecorderKind, metric.ValueObserverKind,
		metric.CounterKind, metric.UpDownCounterKind:
		// Delta-oriented instruments:
		return kind.Includes(CumulativeExporter)

	case metric.SumObserverKind, metric.UpDownSumObserverKind:
		// Cumulative-oriented instruments:
		return kind.Includes(DeltaExporter)
	}
	// Something unexpected is happening--we could panic.  This
	// will become an error when the exporter tries to access a
	// checkpoint, presumably, so let it be.
	return false
}
