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
				{{ $t('No repos with a docker-compose.yml/.yaml found.') }}
			</div>

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
		 * such (best-effort, by comparing normalized names, since a repo's
		 * own compose file can declare any app name) rather than hidden, so
		 * it's clear at a glance what was actually found vs what's still new.
		 * @return {*} void
		 */
		async scan() {
			this.isLoading = true
			try {
				const [allRepos, appGrid] = await Promise.all([
					this.$github.listRepos(this.token),
					this.$openAPI.appGrid.getAppGrid().then(res => res.data.data || []),
				])

				// Match on both the app's internal project id (item.name) and
				// its actual displayed title (item.title, a {lang: text} map -
				// what a repo is casually "called" often matches the title a
				// user typed at install time far better than the project id,
				// which can be anything, e.g. auto-generated).
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
					const compose = await this.$github.findComposeFile(this.token, owner, name).catch(() => null)
					if (!compose) return null
					return {
						full_name: repo.full_name,
						compose,
						installed: installedNames.has(this.normalize(repo.name)),
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
