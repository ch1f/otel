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

package httptrace

import (
	"context"
	"net/http"

	"github.com/Ch1f/otel/api/correlation"
	"github.com/Ch1f/otel/api/global"
	"github.com/Ch1f/otel/api/kv"
	"github.com/Ch1f/otel/api/propagation"
	"github.com/Ch1f/otel/api/standard"
	"github.com/Ch1f/otel/api/trace"
)

// Option is a function that allows configuration of the httptrace Extract()
// and Inject() functions
type Option func(*config)

type config struct {
	propagators propagation.Propagators
}

func newConfig(opts []Option) *config {
	c := &config{propagators: global.Propagators()}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WithPropagators sets the propagators to use for Extraction and Injection
func WithPropagators(props propagation.Propagators) Option {
	return func(c *config) {
		c.propagators = props
	}
}

// Returns the Attributes, Context Entries, and SpanContext that were encoded by Inject.
func Extract(ctx context.Context, req *http.Request, opts ...Option) ([]kv.KeyValue, []kv.KeyValue, trace.SpanContext) {
	c := newConfig(opts)
	ctx = propagation.ExtractHTTP(ctx, c.propagators, req.Header)

	attrs := append(
		standard.HTTPServerAttributesFromHTTPRequest("", "", req),
		standard.NetAttributesFromHTTPRequest("tcp", req)...,
	)

	var correlationCtxKVs []kv.KeyValue
	correlation.MapFromContext(ctx).Foreach(func(kv kv.KeyValue) bool {
		correlationCtxKVs = append(correlationCtxKVs, kv)
		return true
	})

	return attrs, correlationCtxKVs, trace.RemoteSpanContextFromContext(ctx)
}

func Inject(ctx context.Context, req *http.Request, opts ...Option) {
	c := newConfig(opts)
	propagation.InjectHTTP(ctx, c.propagators, req.Header)
}
