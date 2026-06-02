package simulator

import "fmt"

type ConnectorState string

const (
	StateAvailable     ConnectorState = "Available"
	StatePreparing     ConnectorState = "Preparing"
	StateCharging      ConnectorState = "Charging"
	StateSuspendedEV   ConnectorState = "SuspendedEV"
	StateSuspendedEVSE ConnectorState = "SuspendedEVSE"
	StateFinishing     ConnectorState = "Finishing"
	StateReserved      ConnectorState = "Reserved"
	StateUnavailable   ConnectorState = "Unavailable"
	StateFaulted       ConnectorState = "Faulted"
)

var allowedTransitions = map[ConnectorState][]ConnectorState{
	StateAvailable:     {StatePreparing, StateUnavailable, StateFaulted},
	StatePreparing:     {StateCharging, StateAvailable, StateFaulted},
	StateCharging:      {StateSuspendedEV, StateSuspendedEVSE, StateFinishing, StateFaulted},
	StateSuspendedEV:   {StateCharging, StateFinishing, StateFaulted},
	StateSuspendedEVSE: {StateCharging, StateFinishing, StateFaulted},
	StateFinishing:     {StateAvailable, StateFaulted},
	StateUnavailable:   {StateAvailable, StateFaulted},
	StateReserved:      {StateAvailable, StateFaulted},
	StateFaulted:       {StateAvailable},
}

type ConnectorStateMachine struct {
	current ConnectorState
}

func NewConnectorStateMachine(initial ConnectorState) *ConnectorStateMachine {
	return &ConnectorStateMachine{current: initial}
}

func (m *ConnectorStateMachine) Current() ConnectorState {
	return m.current
}

func (m *ConnectorStateMachine) Transition(to ConnectorState) error {
	if m.current == to {
		return nil
	}
	allowed, ok := allowedTransitions[m.current]
	if !ok {
		return fmt.Errorf("no transitions defined from %s", m.current)
	}
	for _, s := range allowed {
		if s == to {
			m.current = to
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s -> %s", m.current, to)
}

func (m *ConnectorStateMachine) CanTransitionTo(to ConnectorState) bool {
	if m.current == to {
		return true
	}
	allowed, ok := allowedTransitions[m.current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
