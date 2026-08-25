// Shared by the "Installable from GitHub" widget (which scans a connected
// account's own repos) and the main "+" menu's "Install from GitHub" entry
// (which installs from any repo, by URL) - both end up needing the same
// "turn a repo into an install-ready compose YAML" logic, just starting
// from a different place (a scanned repo object vs a pasted URL).
import YAML from 'yaml'
import github from '@/service/github.js'

/**
 * @description: Accepts a full URL (https://github.com/owner/repo,
 * optionally with a trailing path/query), a bare github.com/owner/repo, or
 * just owner/repo.
 * @param {string} input
 * @return {{owner: string, repo: string}|null}
 */
export function parseRepoUrl(input) {
	const trimmed = (input || '').trim()
	const match = trimmed.match(/^(?:https?:\/\/)?(?:www\.)?github\.com\/([^/\s]+)\/([^/\s#?]+)/i)
		|| trimmed.match(/^([^/\s]+)\/([^/\s]+)$/)
	if (!match) return null
	return { owner: match[1], repo: match[2].replace(/\.git$/, '') }
}

// Whether any service builds from source (build:) instead of referencing a
// prebuilt image - such a repo has no runnable image on its own, only the
// one fetched compose YAML, so installing it needs the actual repo
// contents fetched too (see taggedForInstall below).
function hasBuildService(composeYaml) {
	try {
		const parsed = YAML.parse(composeYaml)
		const services = parsed?.services || {}
		return Object.values(services).some(service => service && service.build)
	}
	catch (error) {
		return false
	}
}

/**
 * @description: Tags a build:-based compose file with where its source
 * actually lives, so the install pipeline can clone it before building -
 * see AppManagement's fetchSourceRepoIfNeeded. Returns compose unchanged
 * for a repo that references prebuilt images, since there's nothing to
 * fetch beyond the compose file itself.
 * @param {string} composeYaml
 * @param {string} fullName "owner/repo"
 * @param {string} defaultBranch
 * @return {string}
 */
export function taggedForInstall(composeYaml, fullName, defaultBranch) {
	if (!hasBuildService(composeYaml)) return composeYaml
	try {
		const parsed = YAML.parse(composeYaml)
		const [owner, name] = fullName.split('/')
		parsed['x-casaos'] = parsed['x-casaos'] || {}
		parsed['x-casaos'].source_repo = `git::https://github.com/${owner}/${name}.git?ref=${defaultBranch}`
		return YAML.stringify(parsed)
	}
	catch (error) {
		// Fall through with the original compose text - install will just
		// fail downstream the same way it always did for a build-based
		// repo, rather than silently dropping the caller's request.
		return composeYaml
	}
}

/**
 * @description: Resolves any repo (URL, github.com/owner/repo, or
 * owner/repo - not limited to ones the token owns or collaborates on) to
 * an install-ready compose YAML string. Returns an error code instead of
 * throwing, since "no compose file found" vs "repo not found" vs a
 * malformed input each need their own wording depending on where this is
 * called from (widget vs a dialog prompt).
 * @param {string} token
 * @param {string} input
 * @return {Promise<{compose?: string, error?: 'invalid_url'|'not_found'|'unreadable'|'no_compose'}>}
 */
export async function resolveInstallableCompose(token, input) {
	const parsed = parseRepoUrl(input)
	if (!parsed) return { error: 'invalid_url' }

	let repoInfo
	try {
		repoInfo = await github.getRepo(token, parsed.owner, parsed.repo)
	}
	catch (error) {
		return { error: error.response && error.response.status === 404 ? 'not_found' : 'unreadable' }
	}

	const compose = await github.findComposeFile(token, parsed.owner, parsed.repo, repoInfo.default_branch)
	if (!compose) return { error: 'no_compose' }

	return { compose: taggedForInstall(compose, repoInfo.full_name, repoInfo.default_branch) }
}
