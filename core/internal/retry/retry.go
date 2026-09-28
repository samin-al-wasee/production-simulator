// Package retry models message retry and dead-letter semantics: exponential
// backoff, a bounded number of attempts, and the point where a message is
// dead-lettered. It is pure and deterministic; the same policy always yields
// the same schedule, so it can back both simulations and assertions about the
// messaging components (Phase 3).
package retry

import (
	"fmt"
	"time"
)

// Policy describes retry behavior for a consumer.
type Policy struct {
	// MaxAttempts is the total number of delivery attempts, including the first.
	MaxAttempts int
	// BaseDelay is the wait before the first retry.
	BaseDelay time.Duration
	// Factor multiplies the delay after every retry (>= 1).
	Factor float64
	// MaxDelay caps the delay; zero means no cap.
	MaxDelay time.Duration
}

// Validate reports whether the policy is usable.
func (p Policy) Validate() error {
	switch {
	case p.MaxAttempts < 1:
		return fmt.Errorf("max attempts must be >= 1, got %d", p.MaxAttempts)
	case p.BaseDelay < 0:
		return fmt.Errorf("base delay must not be negative")
	case p.Factor < 1:
		return fmt.Errorf("factor must be >= 1, got %v", p.Factor)
	case p.MaxDelay < 0:
		return fmt.Errorf("max delay must not be negative")
	}
	return nil
}

// Delay returns the wait before retry number n (1 is the first retry).
func (p Policy) Delay(n int) time.Duration {
	if n < 1 {
		return 0
	}
	d := float64(p.BaseDelay)
	for i := 1; i < n; i++ {
		d *= p.Factor
		if p.MaxDelay > 0 && d >= float64(p.MaxDelay) {
			return p.MaxDelay
		}
	}
	if p.MaxDelay > 0 && d > float64(p.MaxDelay) {
		return p.MaxDelay
	}
	return time.Duration(d)
}

// Action is what a consumer does with a failed delivery.
type Action int

const (
	// Retry redelivers the message after Decision.Delay.
	Retry Action = iota
	// DeadLetter moves the message to the dead-letter queue.
	DeadLetter
)

func (a Action) String() string {
	if a == Retry {
		return "retry"
	}
	return "dead-letter"
}

// Decision is the outcome of a failed delivery.
type Decision struct {
	Action Action
	Delay  time.Duration
}

// Decide returns what happens after delivery attempt number `attempt`
// (1-based) fails. Attempts at or beyond MaxAttempts are dead-lettered.
func (p Policy) Decide(attempt int) Decision {
	if attempt >= p.MaxAttempts {
		return Decision{Action: DeadLetter}
	}
	return Decision{Action: Retry, Delay: p.Delay(attempt)}
}

// Outcome is the full history of one message.
type Outcome struct {
	Attempts     int
	Delays       []time.Duration
	DeadLettered bool
	// Total is the elapsed time from the first attempt to the final one.
	Total time.Duration
}

// Simulate plays out a message whose first `failures` attempts fail. A
// message that fails MaxAttempts times is dead-lettered.
func Simulate(p Policy, failures int) (Outcome, error) {
	if err := p.Validate(); err != nil {
		return Outcome{}, err
	}
	if failures < 0 {
		return Outcome{}, fmt.Errorf("failures must be >= 0, got %d", failures)
	}
	var out Outcome
	for attempt := 1; ; attempt++ {
		out.Attempts = attempt
		if attempt > failures {
			return out, nil
		}
		d := p.Decide(attempt)
		if d.Action == DeadLetter {
			out.DeadLettered = true
			return out, nil
		}
		out.Delays = append(out.Delays, d.Delay)
		out.Total += d.Delay
	}
}
