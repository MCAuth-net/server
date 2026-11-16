package helpers

import (
	"context"
	"time"
)

func GetContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
