<script>
export default {
	name: 'AutoUpdateModal',
	data() {
		return {
			isLoading: true,
			apps: [],
			rechecking: null,
			search: '',
			runningAll: false,
		}
	},
	computed: {
		autoUpdateCount() {
			return this.apps.filter(app => app.autoUpdate).length
		},
		visibleApps() {
			const query = this.search.trim().toLowerCase()
			const filtered = query
				? this.apps.filter(app => app.displayName.toLowerCase().includes(query))
				: this.apps
			return [...filtered].sort((a, b) => a.displayName.localeCompare(b.displayName))
		},
	},
	created() {
		this.load()
		// pick up a pass that was already running when the panel opened
		this.$api.autoupdate.runStatus().then((res) => {
			if (res.data.data.running)
				this.watchRun()
		}).catch(() => {})
	},
	beforeDestroy() {
		clearInterval(this.runTimer)
	},
	methods: {
		async load() {
			this.isLoading = true
			try {
				const res = await this.$api.autoupdate.listApps()
				this.apps = res.data.data || []
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not load auto-update settings.'), type: 'is-danger' })
			}
			finally {
				this.isLoading = false
			}
		},

		async setSettings(app, changes) {
			const previous = { autoUpdate: app.autoUpdate, notify: app.notify }
			Object.assign(app, changes)
			try {
				await this.$api.autoupdate.setSettings(app.name, { autoUpdate: app.autoUpdate, notify: app.notify })
			}
			catch (error) {
				Object.assign(app, previous)
				this.$buefy.toast.open({ message: this.$t('Could not save settings for {name}.', { name: app.name }), type: 'is-danger' })
			}
		},

		confirmRunAll() {
			this.$buefy.dialog.confirm({
				message: this.$t('This checks every app now and updates the {count} with Auto-update on. Those containers will restart, and it can take a few minutes.', { count: this.autoUpdateCount }),
				confirmText: this.$t('Check and update'),
				cancelText: this.$t('Cancel'),
				onConfirm: () => this.runAll(),
			})
		},

		async runAll() {
			try {
				await this.$api.autoupdate.runAll()
			}
			catch (error) {
				if (error.response?.status !== 409) {
					this.$buefy.toast.open({ message: this.$t('Could not start the update check.'), type: 'is-danger' })
					return
				}
				// 409: a pass is already going - just follow it
			}
			this.watchRun()
		},

		// follow a pass in progress: refresh the rows as it goes (each app's
		// status is updated as soon as it has been checked) and say when done
		watchRun() {
			this.runningAll = true
			clearInterval(this.runTimer)
			this.runTimer = setInterval(async () => {
				try {
					const res = await this.$api.autoupdate.runStatus()
					const running = res.data.data.running
					const list = await this.$api.autoupdate.listApps()
					this.apps = list.data.data || []
					if (!running) {
						clearInterval(this.runTimer)
						this.runningAll = false
						this.$buefy.toast.open({ message: this.$t('Finished checking and updating your apps.'), type: 'is-success' })
					}
				}
				catch (error) {
					clearInterval(this.runTimer)
					this.runningAll = false
				}
			}, 4000)
		},

		async recheck(app) {
			this.rechecking = app.name
			try {
				const res = await this.$api.autoupdate.recheck(app.name)
				Object.assign(app, res.data.data)
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not check {name} for updates.', { name: app.name }), type: 'is-danger' })
			}
			finally {
				this.rechecking = null
			}
		},
	},
}
</script>

<template>
	<div class="modal-card autoupdate-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Auto-Update') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Checks each app\'s image against its registry for a newer version. Notify and Auto-update are independent - nothing auto-updates unless you check that box, and you can leave notifications on or off separately.') }}
			</p>

			<div v-if="apps.length > 0" class="is-flex is-align-items-center mb-4">
				<b-input
					v-model="search" :placeholder="$t('Search apps…')"
					icon="magnify" class="is-flex-grow-1 mr-3" size="is-small" expanded
				/>
				<b-button
					size="is-small" rounded type="is-primary" icon-pack="casa" icon-left="restart-outline"
					:loading="runningAll" :disabled="runningAll" @click="confirmRunAll"
				>
					{{ runningAll ? $t('Checking…') : $t('Check and update all') }}
				</b-button>
			</div>

			<b-loading v-model="isLoading" :is-full-page="false" />

			<template v-if="!isLoading">
				<div v-if="apps.length === 0" class="has-text-grey-100 is-size-7 py-2">
					{{ $t('No managed apps found.') }}
				</div>
				<div v-else-if="visibleApps.length === 0" class="has-text-grey-100 is-size-7 py-2">
					{{ $t('No apps match your search.') }}
				</div>

				<div v-for="app in visibleApps" :key="app.appType + ':' + app.name" class="app-row mb-2 p-3">
					<div class="is-flex is-align-items-center">
						<span class="is-flex-grow-1 has-text-weight-semibold one-line" :title="app.currentImage">{{ app.displayName }}</span>
						<b-icon
							class="is-clickable mr-2" icon="restart-outline" pack="casa" size="is-small"
							:class="{ spinning: rechecking === app.name }" @click.native="recheck(app)"
						/>
						<b-checkbox
							:value="app.notify" size="is-small" class="mr-3"
							@input="setSettings(app, { notify: $event })"
						>
							{{ $t('Notify') }}
						</b-checkbox>
						<b-checkbox
							:value="app.autoUpdate" size="is-small"
							@input="setSettings(app, { autoUpdate: $event })"
						>
							{{ $t('Auto-update') }}
						</b-checkbox>
					</div>
					<div class="is-size-7 has-text-grey mt-1">
						<template v-if="app.updateAvailable">
							{{ app.currentTag }} &rarr; <span class="has-text-weight-semibold">{{ app.latestKnownTag }}</span> {{ $t('available') }}
						</template>
						<template v-else-if="app.latestKnownTag">
							{{ $t('Up to date') }} ({{ app.currentTag }})
						</template>
						<template v-else-if="!app.checkError">
							{{ app.currentTag }} - {{ $t('no comparable version tags found') }}
						</template>
					</div>
					<div v-if="app.checkError" class="is-size-7 has-text-danger mt-1">
						{{ app.currentTag }} - {{ app.checkError }}
					</div>
				</div>
			</template>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.autoupdate-modal {
	.modal-card-body {
		min-height: 12rem;
		position: relative;
	}

	.app-row {
		border: 1px solid $border;
		border-radius: 8px;
	}

	.one-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.spinning {
		animation: autoupdate-modal-spin 0.8s linear infinite;
	}

	@keyframes autoupdate-modal-spin {
		from {
			transform: rotate(0deg);
		}

		to {
			transform: rotate(360deg);
		}
	}
}
</style>
