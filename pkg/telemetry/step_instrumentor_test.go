package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/beckn-one/beckn-onix/pkg/model"
	"github.com/stretchr/testify/require"
)

type stubStep struct {
	err error
}

func (s stubStep) Run(ctx *model.StepContext) error {
	return s.err
}

func TestInstrumentedStepSuccess(t *testing.T) {
	ctx := context.Background()
	provider, err := NewProvider(ctx, &Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		EnableMetrics:  true,
		Environment:    "test",
	})
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	step, err := NewInstrumentedStep(stubStep{}, "test-step", "test-module")
	require.NoError(t, err)

	stepCtx := &model.StepContext{
		Context: context.Background(),
		Role:    model.RoleBAP,
	}
	require.NoError(t, step.Run(stepCtx))
}

func TestInstrumentedStepError(t *testing.T) {
	ctx := context.Background()
	provider, err := NewProvider(ctx, &Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		EnableMetrics:  true,
		Environment:    "test",
	})
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	step, err := NewInstrumentedStep(stubStep{err: errors.New("boom")}, "test-step", "test-module")
	require.NoError(t, err)

	stepCtx := &model.StepContext{
		Context: context.Background(),
		Role:    model.RoleBAP,
	}
	require.Error(t, step.Run(stepCtx))
}
