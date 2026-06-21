package subscriptions

// Status is a subscription lifecycle state.
type Status string

const (
	Trialing  Status = "trialing"
	Active    Status = "active"
	PastDue   Status = "past_due"
	Suspended Status = "suspended"
	Canceled  Status = "canceled"
)

// Event drives a transition.
type Event string

const (
	EvPayment      Event = "payment"
	EvPeriodEnd    Event = "period_end"
	EvGraceExpired Event = "grace_expired"
	EvCancel       Event = "cancel"
)

// transitions is the allowed (from,event)->to table. Anything absent is illegal.
var transitions = map[Status]map[Event]Status{
	Trialing:  {EvPayment: Active, EvPeriodEnd: PastDue, EvCancel: Canceled},
	Active:    {EvPeriodEnd: PastDue, EvCancel: Canceled},
	PastDue:   {EvPayment: Active, EvGraceExpired: Suspended, EvCancel: Canceled},
	Suspended: {EvPayment: Active, EvCancel: Canceled},
	Canceled:  {}, // terminal
}

// Next returns the destination status for (from,event), or ok=false if illegal.
func Next(from Status, e Event) (Status, bool) {
	to, ok := transitions[from][e]
	return to, ok
}

// AccessAllowed reports whether a status grants product access.
func AccessAllowed(s Status) bool {
	switch s {
	case Trialing, Active, PastDue:
		return true
	default:
		return false
	}
}
