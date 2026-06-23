package base

import (
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Database-connection error classification. These power the boot-time migrate
// retry and the background worker's reconnect loop: billing must survive a
// postgres that is starting late or that gets recreated with a new IP while
// billing is already running.

// IsConfigConnErr reports whether err is a connection that reached postgres but
// was rejected for a configuration reason: bad password or a missing database/
// role. These are still worth retrying (an operator may be fixing the DB), but
// they are logged loudly so a real misconfiguration is never masked as a blip.
func IsConfigConnErr(err error) bool {
	if err == nil {
		return false
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "28P01", // invalid_password
			"28000", // invalid_authorization_specification
			"3D000": // invalid_catalog_name (database does not exist)
			return true
		}
	}
	// golang-migrate flattens errors to strings and drops the typed *PgError,
	// so also match the message golang-migrate/pgx surface.
	m := err.Error()
	for _, s := range []string{
		"password authentication failed",
		"SQLSTATE 28P01",
		"SQLSTATE 28000",
		"SQLSTATE 3D000",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// IsRetryableConnErr reports whether err is a transient inability to reach
// postgres — dial refusal, DNS hiccup, reset, timeout — or a config rejection
// (see IsConfigConnErr). Such errors mean "wait and try again", never "give up".
// A genuine migration/SQL error (e.g. a broken migration) is NOT retryable and
// returns false so it fails fast.
func IsRetryableConnErr(err error) bool {
	if err == nil {
		return false
	}
	if IsConfigConnErr(err) {
		return true
	}
	// Typed paths: pgconn wraps every dial/DNS/timeout failure that happens
	// while establishing a connection in *pgconn.ConnectError.
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// String fallback for errors that lost their type on the way up (notably
	// through golang-migrate). Kept connect-specific so real SQL errors such as
	// `relation "x" does not exist` are not swept in.
	m := err.Error()
	for _, s := range []string{
		"connection refused",
		"no such host",
		"server misbehaving",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"failed to connect",
		"dial tcp",
		"unexpected EOF",
		"connection closed",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}
