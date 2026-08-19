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
		if best == nil || isNewer(v, t, best, bestTag) {
			best, bestTag = v, t
		}
	}

	return bestTag, best != nil
}

// isNewer reports whether (v, t) should be considered newer than the
// current best (best, bestTag). Major.Minor.Patch is unambiguous and
// deferred to semver directly. When those are equal and both sides carry a
// prerelease, this deliberately does NOT trust semver's own prerelease
// ordering: per spec, a single undotted identifier like "rc9" or "rc18"
// compares as an opaque ASCII string, not a number, so "rc9" > "rc18"
// lexicographically (the first differing character is '9' vs '1') even
// though rc18 clearly shipped later. Confirmed live: kinetic sat on
// rc18 and got reported an "update" to rc9, an actual downgrade, for
// exactly this reason. Falls back to natural-order comparison of the raw
// tag instead, which sorts embedded digit runs numerically the way a
// human (and every other version-aware tool) expects.
func isNewer(v *semver.Version, t string, best *semver.Version, bestTag string) bool {
	if v.Major() != best.Major() {
		return v.Major() > best.Major()
	}
	if v.Minor() != best.Minor() {
		return v.Minor() > best.Minor()
	}
	if v.Patch() != best.Patch() {
		return v.Patch() > best.Patch()
	}
	if v.Prerelease() == "" || best.Prerelease() == "" {
		// a real release always outranks a prerelease of the same core
		return v.Prerelease() == "" && best.Prerelease() != ""
	}
	return naturalLess(bestTag, t)
}

// naturalLess reports whether a sorts before b in natural order: runs of
// digits compare numerically, everything else compares byte-for-byte -
// "rc9" sorts before "rc18" the way a human expects, unlike a plain
// lexicographic string comparison.
func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ac, bc := a[ai], b[bi]
		if isDigit(ac) && isDigit(bc) {
			aStart, bStart := ai, bi
			for ai < len(a) && isDigit(a[ai]) {
				ai++
			}
			for bi < len(b) && isDigit(b[bi]) {
				bi++
			}
			aNum, bNum := trimLeadingZeros(a[aStart:ai]), trimLeadingZeros(b[bStart:bi])
			if len(aNum) != len(bNum) {
				return len(aNum) < len(bNum)
			}
			if aNum != bNum {
				return aNum < bNum
			}
			continue
		}
		if ac != bc {
			return ac < bc
		}
		ai++
		bi++
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}
