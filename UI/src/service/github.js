// Talks directly to api.github.com from the browser using a user-supplied
// Personal Access Token - this is deliberately a separate plain axios
// instance from service.js's `instance`, which points at this CasaOS
// server's own API and attaches its own auth/base-url handling that must
// not leak into requests aimed at GitHub.
import axios from 'axios'

const github = axios.create({
	baseURL: 'https://api.github.com',
	headers: {
		Accept: 'application/vnd.github+json',
	},
})

const authHeader = token => ({ Authorization: `Bearer ${token}` })

// Newer Docker Compose (v2) convention drops the "docker-" prefix, and both
// forms are still common in the wild, so all four are worth checking.
const COMPOSE_FILENAMES = ['docker-compose.yml', 'docker-compose.yaml', 'compose.yml', 'compose.yaml']

export default {
	/**
	 * @description: Verifies a token works and returns the account it belongs to.
	 * @param {string} token
	 * @return {Promise<string>} the GitHub login name
	 */
	async getUser(token) {
		const res = await github.get('/user', { headers: authHeader(token) })
		return res.data.login
	},

	/**
	 * @description: Lists repos the token can see - own repos and anything
	 * the account collaborates on, private included, most recently updated first.
	 * @param {string} token
	 * @return {Promise<Array>}
	 */
	async listRepos(token) {
		const res = await github.get('/user/repos', {
			headers: authHeader(token),
			params: { per_page: 100, sort: 'updated', affiliation: 'owner,collaborator' },
		})
		return res.data
	},

	/**
	 * @description: Fetches a single repo's metadata (default_branch, in
	 * particular) - for installing from a repo the token doesn't own or
	 * collaborate on (so it wouldn't show up in listRepos), pasted in by
	 * full_name or URL. Works for any repo the token can see, public repos
	 * included regardless of what the token itself is scoped to, since
	 * public repo metadata needs no elevated permission.
	 * @param {string} token
	 * @param {string} owner
	 * @param {string} repo
	 * @return {Promise<Object>}
	 */
	async getRepo(token, owner, repo) {
		const res = await github.get(`/repos/${owner}/${repo}`, { headers: authHeader(token) })
		return res.data
	},

	/**
	 * @description: Fetches a file's raw text content, or null if it doesn't exist.
	 * @param {string} token
	 * @param {string} owner
	 * @param {string} repo
	 * @param {string} path
	 * @return {Promise<string|null>}
	 */
	async getFileContent(token, owner, repo, path) {
		try {
			const res = await github.get(`/repos/${owner}/${repo}/contents/${path}`, {
				headers: { ...authHeader(token), Accept: 'application/vnd.github.raw+json' },
			})
			return typeof res.data === 'string' ? res.data : null
		}
		catch (error) {
			if (error.response && error.response.status === 404) return null
			throw error
		}
	},

	/**
	 * @description: Looks for a compose file anywhere in the repo (not just
	 * the root - personal repos often tuck it into a docker/ or deploy/
	 * subfolder), trying both the docker-compose.y*ml and compose.y*ml
	 * naming conventions. When multiple match, prefers the shallowest path.
	 * @param {string} token
	 * @param {string} owner
	 * @param {string} repo
	 * @param {string} defaultBranch
	 * @return {Promise<string|null>} the file's content, or null if none exist
	 */
	async findComposeFile(token, owner, repo, defaultBranch) {
		let paths
		try {
			const res = await github.get(`/repos/${owner}/${repo}/git/trees/${defaultBranch}`, {
				headers: authHeader(token),
				params: { recursive: 1 },
			})
			paths = (res.data.tree || [])
				.filter(entry => entry.type === 'blob' && COMPOSE_FILENAMES.includes(entry.path.split('/').pop()))
				.map(entry => entry.path)
				.sort((a, b) => a.split('/').length - b.split('/').length)
		}
		catch (error) {
			// A 403 here means the token can list this repo but isn't allowed
			// to read its contents (fine-grained PATs gate that separately
			// under Repository permissions > Contents) - every other call for
			// this repo would fail the same way, so surface it immediately
			// instead of burning more requests on root-only fallback checks.
			if (error.response && error.response.status === 403) throw error
			// Otherwise fall back to root-only checks - covers empty repos (no
			// commits yet, so no tree) and any other tree-listing failure.
			paths = COMPOSE_FILENAMES
		}

		for (const path of paths) {
			const content = await this.getFileContent(token, owner, repo, path)
			if (content) return content
		}
		return null
	},
}
