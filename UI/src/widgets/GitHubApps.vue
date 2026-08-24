<template>
	<div class="widget has-text-white github-apps is-relative">
		<div class="blur-background"></div>
		<div class="widget-content">
			<!-- Header Start -->
			<div class="widget-header is-flex">
				<div class="widget-title is-flex-grow-1">
					{{ $t('Installable from GitHub') }}
				</div>
				<b-icon
					v-if="connected" class="is-clickable" icon="restart-outline" pack="casa"
					size="is-small" :class="{ spinning: isLoading }" @click.native="scan"
				/>
			</div>
			<!-- Header End -->

			<div v-if="!connected" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Connect GitHub in Settings to see repos you can install from.') }}
			</div>
			<div v-else-if="isLoading" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Scanning your repos…') }}
			</div>
			<div v-else-if="tokenLacksContentAccess" class="has-text-danger is-size-7 py-2">
				{{ $t('Your token can list repos but not read their files. Edit it at {url}, enable Contents: Read-only under Repository permissions, then reconnect.', { url: 'github.com/settings/personal-access-tokens' }) }}
			</div>
			<div v-else-if="repos.length === 0" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('No repos with a compose file found.') }}
			</div>

			<!-- Install from someone else's repo Start -->
			<div v-if="connected" class="install-from-url is-flex is-align-items-center mb-2">
				<b-input
					v-model="urlInput" size="is-small" expanded
					:placeholder="$t('Or paste any repo, e.g. owner/repo')" @keyup.native.enter="installFromUrl"
				/>
				<b-button
					size="is-small" rounded type="is-dark" class="ml-2" :loading="installingFromUrl"
					@click="installFromUrl"
				>
					{{ $t('Install') }}
				</b-button>
			</div>
			<!-- Install from someone else's repo End -->

			<div v-for="repo in repos" :key="repo.full_name" class="repo-row mb-2 is-flex is-align-items-center">
				<span class="one-line is-flex-grow-1 is-size-7" :title="repo.full_name">{{ repo.full_name }}</span>
				<span v-if="repo.installed" class="tag is-size-7 ml-2">{{ $t('Installed') }}</span>
				<b-button v-else size="is-small" rounded type="is-dark" class="ml-2" @click="install(repo)">
					{{ $t('Install') }}
				</b-button>
			</div>

			<div v-if="!isLoading && connected && scannedCount > 0" class="has-text-grey-100 is-size-7 mt-2">
				{{ $t('Scanned {count} repos.', { count: scannedCount }) }}
			</div>
		</div>
	</div>
</template>

<script>
import { mixin } from '@/mixins/mixin'
import events from '@/events/events'
import { ice_i18n } from '@/mixins/base/common-i18n'
import YAML from 'yaml'

const githubConfig = 'github_token'
// Caps how many repos get checked for a compose file per scan - each check
// is a separate GitHub API call, so this keeps a refresh well within a
// personal access token's rate limit even for accounts with a lot of repos.
// listRepos itself is already capped at 100 (one page), so this only
// matters for accounts with more repos than that.
const MAX_REPOS_TO_SCAN = 100

export default {
	// eslint-disable-next-line vue/multi-word-component-names
	name: 'githubApps',
	icon: 'github',
	title: 'Installable from GitHub',
	initShow: false,
	mixins: [mixin],
	data() {
		return {
			connected: false,
			token: '',
			isLoading: false,
			repos: [],
			scannedCount: 0,
			tokenLacksContentAccess: false,
			urlInput: '',
			installingFromUrl: false,
		}
	},
	mounted() {
		this.$api.users.getCustomStorage(githubConfig).then((res) => {
			const saved = res.data.data
			if (saved && saved.token) {
				this.connected = true
				this.token = saved.token
				this.scan()
			}
		})
	},
	methods: {
		/**
		 * @description: Scans repos for a docker-compose.yml/.yaml at the
		 * root and lists all of them - installed ones too, just marked as
		 * such rather than hidden, so it's clear at a glance what was
		 * actually found vs what's still new.
		 *
		 * "Already installed" is matched primarily by Docker image (the repo's
		 * compose file's image: values vs each installed app's actual image),
		 * since name-matching turned out unreliable in practice: a compose
		 * project that wasn't given an explicit `name:` gets a random
		 * Docker-generated one (e.g. "clever_khalid"), and even when a repo IS
		 * named deliberately, that name doesn't have to resemble what the repo
		 * is casually called or how the app is titled on the dashboard. Name/
		 * title matching is kept as a fallback for repos whose compose file
		 * doesn't reference a prebuilt image (e.g. uses `build:` instead).
		 * @return {*} void
		 */
		async scan() {
			if (this.isLoading) return
			this.isLoading = true
			this.tokenLacksContentAccess = false
			try {
				const [allRepos, appGrid] = await Promise.all([
					this.$github.listRepos(this.token),
					this.$openAPI.appGrid.getAppGrid().then(res => res.data.data || []),
				])

				const installedImages = new Set(appGrid.map(item => this.imageRepo(item.image)).filter(Boolean))
				const installedNames = new Set()
				appGrid.forEach((item) => {
					installedNames.add(this.normalize(item.name))
					const title = ice_i18n(item.title)
					if (title) installedNames.add(this.normalize(title))
				})

				const candidates = allRepos.slice(0, MAX_REPOS_TO_SCAN)
				this.scannedCount = candidates.length

				const results = await Promise.all(candidates.map(async (repo) => {
					const [owner, name] = repo.full_name.split('/')
					let compose = null
					try {
						compose = await this.$github.findComposeFile(this.token, owner, name, repo.default_branch)
					}
					catch (error) {
						if (error.response && error.response.status === 403) this.tokenLacksContentAccess = true
					}
					if (!compose) return null

					const composeImages = this.imagesFromCompose(compose)
					const installedByImage = composeImages.some(img => installedImages.has(img))
					const installedByName = installedNames.has(this.normalize(repo.name))

					return {
						full_name: repo.full_name,
						default_branch: repo.default_branch,
						compose,
						installed: installedByImage || installedByName,
					}
				}))

				this.repos = results.filter(Boolean)
			}
			catch (error) {
				this.$buefy.toast.open({
					message: this.$t('Could not scan GitHub repos.'),
					type: 'is-danger',
				})
			}
			finally {
				this.isLoading = false
			}
		},

		normalize(name) {
			return (name || '').toLowerCase().replace(/[^a-z0-9]/g, '')
		},

		// Strips the :tag or @digest so "ghcr.io/x/y:1.2.3" and
		// "ghcr.io/x/y:latest" are recognized as the same image.
		imageRepo(image) {
			if (!image) return ''
			return image.split('@')[0].split(':')[0]
		},

		// Pulls every service's image: out of a compose file's text, ignoring
		// services that build from source instead of referencing an image.
		imagesFromCompose(composeYaml) {
			try {
				const parsed = YAML.parse(composeYaml)
				const services = parsed?.services || {}
				return Object.values(services)
					.map(service => this.imageRepo(service?.image))
					.filter(Boolean)
			}
			catch (error) {
				return []
			}
		},

		// Whether any service builds from source (build:) instead of
		// referencing a prebuilt image - such a repo has no runnable image
		// on its own, only findComposeFile's one fetched YAML file, so
		// installing it needs the actual repo contents fetched too (see
		// source_repo below).
		hasBuildService(composeYaml) {
			try {
				const parsed = YAML.parse(composeYaml)
				const services = parsed?.services || {}
				return Object.values(services).some(service => service && service.build)
			}
			catch (error) {
				return false
			}
		},

		install(repo) {
			let compose = repo.compose
			if (this.hasBuildService(compose)) {
				try {
					const parsed = YAML.parse(compose)
					const [owner, name] = repo.full_name.split('/')
					parsed['x-casaos'] = parsed['x-casaos'] || {}
					// Consumed server-side (ComposeService.Install) to clone
					// the repo's actual source into the app's working
					// directory before building - see AppManagement's
					// fetchSourceRepoIfNeeded.
					parsed['x-casaos'].source_repo = `git::https://github.com/${owner}/${name}.git?ref=${repo.default_branch}`
					compose = YAML.stringify(parsed)
				}
				catch (error) {
					// Fall through with the original compose text - install
					// will just fail downstream the same way it always did
					// for a build-based repo, rather than silently dropping
					// the user's click.
				}
			}
			this.$EventBus.$emit(events.SHOW_CUSTOM_INSTALL_WITH_COMPOSE, compose)
		},

		// Accepts a full URL (https://github.com/owner/repo, optionally with
		// a trailing path/query), a bare github.com/owner/repo, or just
		// owner/repo.
		parseRepoUrl(input) {
			const trimmed = (input || '').trim()
			const match = trimmed.match(/^(?:https?:\/\/)?(?:www\.)?github\.com\/([^/\s]+)\/([^/\s#?]+)/i)
				|| trimmed.match(/^([^/\s]+)\/([^/\s]+)$/)
			if (!match) return null
			return { owner: match[1], repo: match[2].replace(/\.git$/, '') }
		},

		// Installs from any repo, not just ones the connected account owns
		// or collaborates on (which is all scan() ever sees) - reuses the
		// exact same findComposeFile/install path scan() results go
		// through, just fetches the one repo's metadata (default_branch)
		// directly instead of pulling it from listRepos.
		async installFromUrl() {
			if (this.installingFromUrl) return
			const parsed = this.parseRepoUrl(this.urlInput)
			if (!parsed) {
				this.$buefy.toast.open({
					message: this.$t('Paste a GitHub repo URL, like github.com/owner/repo.'),
					type: 'is-danger',
				})
				return
			}

			this.installingFromUrl = true
			try {
				const repoInfo = await this.$github.getRepo(this.token, parsed.owner, parsed.repo)
				const compose = await this.$github.findComposeFile(this.token, parsed.owner, parsed.repo, repoInfo.default_branch)
				if (!compose) {
					this.$buefy.toast.open({
						message: this.$t('No compose file found in that repo.'),
						type: 'is-danger',
					})
					return
				}
				this.install({ full_name: repoInfo.full_name, default_branch: repoInfo.default_branch, compose })
				this.urlInput = ''
			}
			catch (error) {
				this.$buefy.toast.open({
					message: error.response && error.response.status === 404
						? this.$t('Repo not found - check the URL and that it\'s public.')
						: this.$t('Could not read that repo.'),
					type: 'is-danger',
				})
			}
			finally {
				this.installingFromUrl = false
			}
		},
	},
}
</script>

<style lang="scss">
.github-apps {
	.repo-row {
		.one-line {
			overflow: hidden;
			text-overflow: ellipsis;
			white-space: nowrap;
		}

		.tag {
			background: hsla(0, 0%, 100%, 0.08);
			color: $grey-100;
			flex-shrink: 0;
		}
	}

	.spinning {
		animation: github-apps-spin 1s linear infinite;
	}

	@keyframes github-apps-spin {
		from {
			transform: rotate(0deg);
		}

		to {
			transform: rotate(360deg);
		}
	}
}
</style>
