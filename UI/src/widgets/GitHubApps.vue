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
			<div v-else-if="repos.length === 0" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Nothing new - every repo with a compose file is already installed.') }}
			</div>

			<div v-for="repo in repos" :key="repo.full_name" class="repo-row mb-2 is-flex is-align-items-center">
				<span class="one-line is-flex-grow-1 is-size-7" :title="repo.full_name">{{ repo.full_name }}</span>
				<b-button size="is-small" rounded type="is-dark" class="ml-2" @click="install(repo)">
					{{ $t('Install') }}
				</b-button>
			</div>
		</div>
	</div>
</template>

<script>
import { mixin } from '@/mixins/mixin'
import events from '@/events/events'

const githubConfig = 'github_token'
// Only the most recently-updated repos are scanned, since checking each one
// for a compose file is a separate GitHub API call - this keeps a refresh
// well within a personal access token's rate limit even for accounts with
// a lot of repos.
const MAX_REPOS_TO_SCAN = 30

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
		 * @description: Scans the most recently updated repos for a
		 * docker-compose.yml/.yaml at the root, and drops anything that looks
		 * like it's already installed - best-effort by comparing normalized
		 * names, since a repo's own compose file can declare any app name.
		 * @return {*} void
		 */
		async scan() {
			this.isLoading = true
			try {
				const [allRepos, appGrid] = await Promise.all([
					this.$github.listRepos(this.token),
					this.$openAPI.appGrid.getAppGrid().then(res => res.data.data || []),
				])

				const installedNames = new Set(appGrid.map(item => this.normalize(item.name)))
				const candidates = allRepos
					.filter(repo => !installedNames.has(this.normalize(repo.name)))
					.slice(0, MAX_REPOS_TO_SCAN)

				const results = await Promise.all(candidates.map(async (repo) => {
					const [owner, name] = repo.full_name.split('/')
					const compose = await this.$github.findComposeFile(this.token, owner, name).catch(() => null)
					return compose ? { full_name: repo.full_name, compose } : null
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

		install(repo) {
			this.$EventBus.$emit(events.SHOW_CUSTOM_INSTALL_WITH_COMPOSE, repo.compose)
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
