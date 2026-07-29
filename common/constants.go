package common

const (
	SERVICENAME = "casaos"
	// This is a personal fork (github.com/cvd-unmatched/casa-os), not an
	// official IceWhale build. IsNeedUpdate (pkg/utils/version/version.go)
	// compares this against the version reported by IceWhale's own
	// api.casaos.io, purely by numeric segment. Deliberately set higher than
	// any real upstream release so CasaOS's built-in "Settings > Update"
	// never offers to "update" this fork back to stock CasaOS and overwrite
	// it - update this fork by pulling from its own repo instead.
	VERSION   = "999.0.0"
	BODY      = " "
	RANW_NAME = "IceWhale-RemoteAccess"
)

// ForkVersion is this fork's own release tag (e.g. "v1.0.6"), set via
// -ldflags at build time by .github/workflows/release.yml. Deliberately a
// separate value from VERSION above - VERSION has to stay pinned at 999.0.0
// to defuse the stock updater, so it can't also be used to track which of
// THIS fork's releases is actually installed. Empty when built any other way
// (e.g. a local `go build`).
var ForkVersion = ""
