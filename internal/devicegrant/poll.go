package devicegrant

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// slowDownStep is what RFC 8628 §3.5 requires: on slow_down the client adds
// five seconds to its interval, for this request and every request after it.
// Ignoring it is how a client polls itself into a rate limit and then reports
// the resulting refusal as a failed login.
const slowDownStep = 5 * time.Second

// schedule is the polling clock. The interval lives here rather than being
// recomputed from each response because slow_down is cumulative: two of them
// mean ten seconds more, not five. step is a field so a test can prove that
// arithmetic without sleeping through it.
type schedule struct {
	interval time.Duration
	step     time.Duration
	deadline time.Time
}

// wait sleeps out one polling interval and reports whether there was still time
// left to poll at all.
func (s *schedule) wait(ctx context.Context) bool {
	if !time.Now().Before(s.deadline) {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(s.interval):
		return true
	}
}

// slower applies RFC 8628 §3.5's slow_down to every request from here on.
func (s *schedule) slower() { s.interval += s.step }

// AwaitToken polls the token endpoint until the user approves, refuses, or runs
// out of time, backing off by five seconds every time the provider answers
// slow_down.
func (p Provider) AwaitToken(ctx context.Context, clientID string, auth Authorization) (string, error) {
	return p.await(ctx, clientID, auth, &schedule{
		interval: auth.Interval,
		step:     slowDownStep,
		deadline: time.Now().Add(auth.Expires),
	})
}

// await is AwaitToken with the clock supplied. A transport error is retried
// rather than fatal: the deadline already bounds the loop, and a dropped packet
// on the fourth poll is no reason to make somebody go and type a new code.
func (p Provider) await(ctx context.Context, clientID string, auth Authorization, poll *schedule) (string, error) {
	for poll.wait(ctx) {
		status, doc, err := p.postForm(ctx, p.Token, url.Values{
			"grant_type":  {GrantType},
			"device_code": {auth.DeviceCode},
			"client_id":   {clientID},
		})
		if err != nil {
			continue
		}
		token, slower, err := read(status, doc)
		switch {
		case err != nil:
			return "", err
		case token != "":
			return token, nil
		case slower:
			poll.slower()
		}
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "", fmt.Errorf("the code expired after %s without being approved — run `sonde login` again",
		auth.Expires.Round(time.Second))
}

// read turns one poll into the three things that can happen: the grant
// completed, keep waiting, or stop. RFC 8628 §3.5's errors are not
// interchangeable — telling somebody their code expired when they in fact
// refused it sends them to retry the thing they meant to stop.
func read(status int, doc map[string]any) (string, bool, error) {
	if status >= 200 && status <= 299 {
		token := text(doc, "access_token")
		if token == "" {
			return "", false, fmt.Errorf("the provider approved this machine but returned no access token")
		}
		return token, false, nil
	}

	switch text(doc, "error") {
	case "authorization_pending":
		return "", false, nil
	case "slow_down":
		return "", true, nil
	case "access_denied":
		return "", false, fmt.Errorf(
			"the sign-in was refused at the provider — run `sonde login` again if that was not deliberate")
	case "expired_token":
		return "", false, fmt.Errorf("the provider expired the code before it was approved — run `sonde login` again")
	}
	return "", false, fmt.Errorf("the provider refused the device grant (%d: %s)", status, refusal(doc))
}
