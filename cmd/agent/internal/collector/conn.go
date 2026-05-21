package collector

// Connections returns the host-wide TCP + UDP connection counts. Implemented
// as a thin facade over ConnConnections so the aggregator can call it through
// the conventional "every collector is one function" shape used by the rest
// of the package.
func Connections() (tcp, udp uint64, err error) {
	return ConnConnections()
}
