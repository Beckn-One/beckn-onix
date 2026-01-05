package definition

import (
	"context"
	"net/http"
)


type OndcWorkbench interface {
	WorkbenchReceiver(context.Context,*http.Request,[]byte) (error)
	WorkbenchProcessor(context.Context,*http.Request,[]byte) (error)
}

type OndcWorkbenchProvider interface {
	New(context.Context,Cache,KeyManager,map[string]string) (OndcWorkbench, func() error, error)
}