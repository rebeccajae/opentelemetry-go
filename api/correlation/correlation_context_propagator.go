package correlation

import (
	"context"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/propagation"
)

// CorrelationContextHeader is specified by W3C.
//nolint:golint
var CorrelationContextHeader = "Correlation-Context"

// CorrelationContext propagates Key:Values in W3C CorrelationContext
// format.
// nolint:golint
type CorrelationContext struct{}

var _ propagation.HTTPPropagator = CorrelationContext{}

// DefaultHTTPPropagator returns the default context correlation HTTP
// propagator.
func DefaultHTTPPropagator() propagation.HTTPPropagator {
	return CorrelationContext{}
}

// Inject implements HTTPInjector.
func (CorrelationContext) Inject(ctx context.Context, supplier propagation.HTTPSupplier) {
	correlationCtx := MapFromContext(ctx)
	firstIter := true
	var headerValueBuilder strings.Builder
	correlationCtx.Foreach(func(kv core.KeyValue) bool {
		if !firstIter {
			headerValueBuilder.WriteRune(',')
		}
		firstIter = false
		headerValueBuilder.WriteString(url.QueryEscape(strings.TrimSpace((string)(kv.Key))))
		headerValueBuilder.WriteRune('=')
		headerValueBuilder.WriteString(url.QueryEscape(strings.TrimSpace(kv.Value.Emit())))
		return true
	})
	if headerValueBuilder.Len() > 0 {
		headerString := headerValueBuilder.String()
		supplier.Set(CorrelationContextHeader, headerString)
	}
}

// Extract implements HTTPExtractor.
func (CorrelationContext) Extract(ctx context.Context, supplier propagation.HTTPSupplier) context.Context {
	correlationContext := supplier.Get(CorrelationContextHeader)
	if correlationContext == "" {
		return ContextWithMap(ctx, NewEmptyMap())
	}

	contextValues := strings.Split(correlationContext, ",")
	keyValues := make([]core.KeyValue, 0, len(contextValues))
	for _, contextValue := range contextValues {
		valueAndProps := strings.Split(contextValue, ";")
		if len(valueAndProps) < 1 {
			continue
		}
		nameValue := strings.Split(valueAndProps[0], "=")
		if len(nameValue) < 2 {
			continue
		}
		name, err := url.QueryUnescape(nameValue[0])
		if err != nil {
			continue
		}
		trimmedName := strings.TrimSpace(name)
		value, err := url.QueryUnescape(nameValue[1])
		if err != nil {
			continue
		}
		trimmedValue := strings.TrimSpace(value)

		// TODO (skaris): properties defiend https://w3c.github.io/correlation-context/, are currently
		// just put as part of the value.
		var trimmedValueWithProps strings.Builder
		trimmedValueWithProps.WriteString(trimmedValue)
		for _, prop := range valueAndProps[1:] {
			trimmedValueWithProps.WriteRune(';')
			trimmedValueWithProps.WriteString(prop)
		}

		keyValues = append(keyValues, key.New(trimmedName).String(trimmedValueWithProps.String()))
	}
	return ContextWithMap(ctx, NewMap(MapUpdate{
		MultiKV: keyValues,
	}))
}

// GetAllKeys implements HTTPPropagator.
func (CorrelationContext) GetAllKeys() []string {
	return []string{CorrelationContextHeader}
}
