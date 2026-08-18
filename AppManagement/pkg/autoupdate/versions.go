package autoupdate

import "github.com/Masterminds/semver/v3"

// NewestTag scans candidateTags (as returned by docker.GetTags) and returns
// the highest-versioned tag using semver ordering, plus true if at least one
// candidate parsed as semver. Tags that don't parse (latest, main, sha-*,
// branch names, arbitrary hashes) are silently skipped, not treated as
// errors - a registry listing mostly-unparseable tags is normal (e.g. CI
// commit-tagged images) and must not abort the whole check. If zero
// candidates parse, ok is false and the caller must treat that as "no
// comparable newer tag found", not fall back to any digest-based guess.
//
// includePrerelease gates whether prerelease tags (v0.0.0-rc16) are
// eligible candidates at all. Callers should only pass true when the app's
// currently pinned tag is itself already a prerelease - this lets rc-track
// images auto-update along their own line (rc16 -> rc18) while never
// surprise-promoting a stable-pinned app onto an unreleased prerelease that
// happens to also exist in the registry.
func NewestTag(candidateTags []string, includePrerelease bool) (tag string, ok bool) {
	var best *semver.Version
	var bestTag string

	for _, t := range candidateTags {
		v, err := semver.NewVersion(t)
		if err != nil {
			continue
		}
		if v.Prerelease() != "" && !includePrerelease {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best, bestTag = v, t
		}
	}

	return bestTag, best != nil
}
