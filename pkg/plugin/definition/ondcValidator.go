package definition

import (
	"context"
	"net/url"
)

type OndcValidator interface {
	ValidatePayload(ctx context.Context, url *url.URL, payload []byte) error
	SaveValidationData(ctx context.Context, url *url.URL, payload []byte) error
}

type OndcValidatorProvider interface {
	New(context.Context,Cache,map[string]string) (OndcValidator, func() error, error)
}