package events

import (
	"sync"
)

// eventSeparator is the separator used to separate module from event name.
const eventSeparator = "."

// registeredEvents stores registered event names.
var registeredEvents sync.Map

// NewEventName creates a new event name.
//
// Module and event must be unique. If they are not, this method will panic.
// removed func

// Provable is an interface implemented by event types which can be proven.
// removed interface

// TypedAttribute is an interface implemented by types which can be transparently used as event
// attributes with CBOR-marshalled value.
// removed interface

// CustomTypedAttribute is an interface implemented by types which can be transparently used as event
// attributes with custom value encoding.
// removed interface

// IsAttributeKind checks whether the given attribute key corresponds to the passed typed attribute.
// removed func

// DecodeValue decodes the attribute event value.
// removed func

// EncodeValue encodes the attribute event value.
// removed func
