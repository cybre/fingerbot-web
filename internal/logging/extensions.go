package logging

import (
	"fmt"
	"log/slog"
)

func ErrAttr(err error) slog.Attr {
	return slog.Any("error", err)
}

func HexAttr(key string, value any) slog.Attr {
	return slog.String(key, fmt.Sprintf("%X", value))
}
