package subscriptions

import "testing"

func TestTransitions(t *testing.T) {
	cases := []struct {
		from  Status
		event Event
		want  Status
		ok    bool
	}{
		{Trialing, EvPayment, Active, true},
		{Trialing, EvPeriodEnd, PastDue, true},
		{Active, EvPeriodEnd, PastDue, true},
		{PastDue, EvPayment, Active, true},
		{PastDue, EvGraceExpired, Suspended, true},
		{Suspended, EvPayment, Active, true},
		{Active, EvCancel, Canceled, true},
		{Trialing, EvCancel, Canceled, true},
		{Suspended, EvCancel, Canceled, true},
		// forbidden
		{Canceled, EvPayment, Canceled, false},
		{Active, EvGraceExpired, Active, false},
		{Suspended, EvPeriodEnd, Suspended, false},
	}
	for _, tc := range cases {
		got, ok := Next(tc.from, tc.event)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("Next(%s,%s) = (%s,%v), want (%s,%v)", tc.from, tc.event, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAccessAllowed(t *testing.T) {
	allowed := map[Status]bool{Trialing: true, Active: true, PastDue: true, Suspended: false, Canceled: false}
	for s, want := range allowed {
		if AccessAllowed(s) != want {
			t.Errorf("AccessAllowed(%s) = %v, want %v", s, AccessAllowed(s), want)
		}
	}
}
