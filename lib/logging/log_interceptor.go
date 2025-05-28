package logging

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/go-errors/errors"
)

func NewErrorLogInterceptor() connect.UnaryInterceptorFunc {
	i := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if err != nil && ctx.Err() == nil {
				var connectErr *connect.Error
				if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInternal {
					var goErr *errors.Error
					if errors.As(err, &goErr) {
						slog.Error("response with internal err",
							"error", fmt.Sprintf("%+v", goErr),
							"stack", goErr.StackFrames(),
						)
					} else {
						slog.Error("response with internal err",
							"error", fmt.Sprintf("%+v", connectErr.Unwrap()),
						)
					}
				}
			}
			return res, err
		})
	}
	return connect.UnaryInterceptorFunc(i)
}
