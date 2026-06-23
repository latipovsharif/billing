package base

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsRetryableConnErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		// transient, typed
		{"dns error", &net.DNSError{Err: "server misbehaving", Name: "billing-postgres"}, true},
		{"net timeout", timeoutErr{}, true},
		// transient, string (golang-migrate flattens these)
		{"migrate refused", errors.New("migrate: failed to open database: failed to connect to `user=postgres database=billing`: dial error: dial tcp 172.19.0.5:5432: connect: connection refused"), true},
		{"worker dns servfail", errors.New("worker begin: failed to connect to `user=postgres`: hostname resolving error: lookup billing-postgres on 127.0.0.11:53: server misbehaving"), true},
		{"no such host", errors.New("dial tcp: lookup billing-postgres: no such host"), true},
		{"reset", errors.New("write tcp: connection reset by peer"), true},
		// config rejections are retryable too (operator may be fixing the DB)
		{"bad password typed", &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}, true},
		{"db missing typed", &pgconn.PgError{Code: "3D000", Message: `database "billing" does not exist`}, true},
		{"bad password string", errors.New(`failed SASL auth: password authentication failed for user "postgres" (SQLSTATE 28P01)`), true},
		// NOT retryable: a real SQL/migration error must fail fast
		{"missing relation", &pgconn.PgError{Code: "42P01", Message: `relation "subscription" does not exist`}, false},
		{"syntax error", &pgconn.PgError{Code: "42601", Message: "syntax error at or near"}, false},
		{"plain logic", errors.New("dirty migration version 7"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsRetryableConnErr(c.err); got != c.want {
				t.Fatalf("IsRetryableConnErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestIsConfigConnErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"bad password typed", &pgconn.PgError{Code: "28P01"}, true},
		{"db missing typed", &pgconn.PgError{Code: "3D000"}, true},
		{"bad password string", errors.New("password authentication failed (SQLSTATE 28P01)"), true},
		{"db missing string", errors.New(`database "billing" does not exist (SQLSTATE 3D000)`), true},
		// transient connect errors are NOT config errors
		{"refused", errors.New("connection refused"), false},
		{"servfail", errors.New("server misbehaving"), false},
		// real SQL error is not a config-connect error
		{"missing relation", &pgconn.PgError{Code: "42P01"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsConfigConnErr(c.err); got != c.want {
				t.Fatalf("IsConfigConnErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// TestWrappedRetryable confirms classification sees through fmt.Errorf %w wraps.
func TestWrappedRetryable(t *testing.T) {
	base := &net.DNSError{Err: "server misbehaving", Name: "billing-postgres"}
	wrapped := fmt.Errorf("worker begin: %w", base)
	if !IsRetryableConnErr(wrapped) {
		t.Fatal("wrapped DNSError should be retryable")
	}
}

// timeoutErr is a net.Error that reports a timeout.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }
