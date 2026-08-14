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

const COMPOSE_FILENAMES = ['docker-compose.yml', 'docker-compose.yaml']

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
	 * @description: Looks for a docker-compose.yml/.yaml at the repo root.
	 * @param {string} token
	 * @param {string} owner
	 * @param {string} repo
	 * @return {Promise<string|null>} the file's content, or null if neither exists
	 */
	async findComposeFile(token, owner, repo) {
		for (const filename of COMPOSE_FILENAMES) {
			const content = await this.getFileContent(token, owner, repo, filename)
			if (content) return content
		}
		return null
	},
}
