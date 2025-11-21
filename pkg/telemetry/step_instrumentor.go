package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"
	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
)

// InstrumentedStep wraps a processing step with telemetry instrumentation.
type InstrumentedStep struct {
	step       definition.Step
	stepName   string
	moduleName string
	metrics    *Metrics
}

// NewInstrumentedStep returns a telemetry enabled wrapper around a definition.Step.
func NewInstrumentedStep(step definition.Step, stepName, moduleName string) (*InstrumentedStep, error) {
	metrics, err := GetMetrics(context.Background())
	if err != nil {
		return nil, err
	}

	return &InstrumentedStep{
		step:       step,
		stepName:   stepName,
		moduleName: moduleName,
		metrics:    metrics,
	}, nil
}

type becknError interface {
	BecknError() *model.Error
}

// Run executes the underlying step and records RED style metrics.
func (is *InstrumentedStep) Run(ctx *model.StepContext) error {
	if is.metrics == nil {
		return is.step.Run(ctx)
	}

	start := time.Now()
	err := is.step.Run(ctx)
	duration := time.Since(start).Seconds()

	attrs := []attribute.KeyValue{
		AttrModule.String(is.moduleName),
		AttrStep.String(is.stepName),
		AttrRole.String(string(ctx.Role)),
	}

	is.metrics.StepExecutionTotal.Add(ctx.Context, 1, metric.WithAttributes(attrs...))
	is.metrics.StepExecutionDuration.Record(ctx.Context, duration, metric.WithAttributes(attrs...))

	if err != nil {
		errorType := fmt.Sprintf("%T", err)
		var becknErr becknError
		if errors.As(err, &becknErr) {
			if be := becknErr.BecknError(); be != nil && be.Code != "" {
				errorType = be.Code
			}
		}

		errorAttrs := append(attrs, AttrErrorType.String(errorType))
		is.metrics.StepErrorsTotal.Add(ctx.Context, 1, metric.WithAttributes(errorAttrs...))
		log.Errorf(ctx.Context, err, "Step %s failed", is.stepName)
	}

	return err
}
