package churp

// removed var block

// CreateEvent is the key manager CHURP create event.
type CreateEvent struct {
	Status *Status
}

// EventKind returns a string representation of this event's kind.
// removed func

// UpdateEvent is the key manager CHURP update event.
type UpdateEvent struct {
	Status *Status
}

// EventKind returns a string representation of this event's kind.
// removed func
