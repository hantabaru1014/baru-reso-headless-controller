package port

// HostDrainer reports whether a host is currently being drained for an
// in-flight auto-upgrade. SessionUsecase consults the drainer before
// starting a new world so that draining hosts cannot accept new sessions
// — otherwise the orchestrator would never observe an empty host and the
// upgrade would stall.
type HostDrainer interface {
	IsHostDraining(hostID string) bool
}

// NoopHostDrainer is a HostDrainer that always reports false. It is used
// by builds (e.g. the CLI) that do not run the upgrade orchestrator.
type NoopHostDrainer struct{}

func (NoopHostDrainer) IsHostDraining(string) bool { return false }
