package function

import (
	"context"

	"github.com/web-seven/overlock/internal/function"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
)

type deleteCmd struct {
	FunctionURL string `arg:"" required:"" help:"Specifies the URL (or multimple comma separated) of function to be deleted from Environment."`
}

func (c *deleteCmd) Run(ctx context.Context, dynamic *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	return function.DeleteFunction(ctx, c.FunctionURL, dynamic, logger)
}
