package sl

import "log/slog"

// Err returns a slog.Attr with the error message
func Err(err error) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}
