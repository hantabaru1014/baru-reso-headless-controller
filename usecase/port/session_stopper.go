package port

import "context"

// SessionStopper stops a running session by id. Defined here so workers
// (specifically the upgrade orchestrator) can reuse SessionUsecase.StopSession
// without importing the usecase package. The single-method interface lets us
// inject it via a setter to break the construction cycle:
//
//	SessionUsecase depends on HostDrainer (= orchestrator)
//	orchestrator depends on SessionStopper (= SessionUsecase)
//
// Wire constructs the orchestrator first (no SessionStopper at constructor
// time) and the linkage is completed in NewServer after wire is done.
type SessionStopper interface {
	StopSession(ctx context.Context, sessionID string) error
}
