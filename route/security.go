package route

import (
	"net"

	"github.com/labstack/echo/v4"
)

// isLoopbackRequest reports whether a request actually arrived over a
// loopback connection, based on the raw TCP peer address rather than
// echo.Context.RealIP(). RealIP() (when no IPExtractor is configured, which
// is the case here) trusts the client-supplied X-Forwarded-For/X-Real-IP
// headers, so a remote attacker could send "X-Forwarded-For: 127.0.0.1" and
// have RealIP() report "127.0.0.1" for a connection that isn't local at all.
// The JWT skipper used to rely on RealIP() for its "is this localhost"
// check, which let any remote, unauthenticated caller bypass auth entirely
// by forging that header. net.Conn's remote address can't be forged this
// way, so it's the only trustworthy source for this decision.
func isLoopbackRequest(c echo.Context) bool {
	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		host = c.Request().RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
