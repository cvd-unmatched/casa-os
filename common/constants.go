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
