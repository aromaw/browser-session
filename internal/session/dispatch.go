package session

import "errors"

// DispatchSupervisor must run before initializing either CLI flags or a GUI.
// Detached supervision therefore works from either binary and outlives its UI.
func DispatchSupervisor(args []string) (bool, error) {
	if len(args) == 0 || args[0] != "__supervise" {
		return false, nil
	}
	if len(args) < 4 {
		return true, errors.New("invalid internal invocation")
	}
	s, err := NewStore(args[1])
	if err != nil {
		return true, err
	}
	return true, s.Supervise(args[2], args[3], args[4:])
}
