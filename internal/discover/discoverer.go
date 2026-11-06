package discover

import "context"

// Discoverer abstracts the source of tailnet devices.
// Implemented by TailscaleDiscoverer and HeadscaleDiscoverer.
type Discoverer interface {
	// List returns all devices known to the tailnet.
	List(ctx context.Context) ([]Device, error)
}
