// Package source defines the LogSource interface and its implementations.
package source

// LogSource is the abstraction for all log input providers.
// Implementations must stream log lines one-by-one through a channel,
// allowing memory-efficient processing of arbitrarily large log files.
type LogSource interface {
	// Stream opens the log source and returns a read-only channel of raw
	// log lines. The channel is closed when all lines have been emitted
	// or an unrecoverable error occurs. Callers should range over the
	// channel until it is closed.
	Stream() (<-chan string, error)
}
