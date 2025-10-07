// filepath: /Users/yaroslavgrebnov/go/src/config/adapters.go
package config

// Generic typed adapters to avoid per-call closure allocations when wiring stages.

func loadFromFileT[T any](p string, t *T) error { return loadFromFile(p, t) }
func writeToFileT[T any](p string, t *T) error  { return writeToFile(p, t) }
