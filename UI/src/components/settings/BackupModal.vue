<script>
import FolderPickerModal from './FolderPickerModal.vue'

export default {
	name: 'BackupModal',
	data() {
		return {
			// export
			exportApps: [],
			exportAppsLoading: true,
			exportExcluded: {}, // name -> true means "don't include this app's data"

			// import
			uploading: false,
			uploadPercent: 0,
			previewing: false,
			preview: null, // { previewId, apps: [...] } once a preview comes back
			confirming: false,
			result: null,
			error: '',
		}
	},
	created() {
		this.loadExportApps()
	},
	methods: {
		loadExportApps() {
			this.exportAppsLoading = true
			this.$api.backup.listApps().then((res) => {
				const apps = res.data.data || []
				apps.sort((a, b) => (b.data_size_bytes || 0) - (a.data_size_bytes || 0))
				this.exportApps = apps
			}).catch(() => {
				this.exportApps = []
			}).finally(() => {
				this.exportAppsLoading = false
			})
		},

		humanSize(bytes) {
			if (!bytes)
				return '0 B'
			const units = ['B', 'KB', 'MB', 'GB', 'TB']
			let n = bytes
			let i = 0
			while (n >= 1024 && i < units.length - 1) {
				n /= 1024
				i++
			}
			return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
		},

		confirmExport() {
			this.$buefy.dialog.confirm({
				title: this.$t('Export'),
				message: this.$t('This downloads every checked app\'s compose config (and data, for apps left checked below) as one archive. It can be very large and take a while. If CasaOS is behind a reverse proxy, a long idle transfer may be cut off by the proxy rather than by CasaOS itself.'),
				confirmText: this.$t('Export'),
				cancelText: this.$t('Cancel'),
				type: 'is-dark',
				onConfirm: () => this.startExport(),
			})
		},

		async startExport() {
			const excluded = this.exportApps.filter(app => this.exportExcluded[app.name]).map(app => app.name)

			// Folder groupings and dashboard order live in a different
			// service AppManagement has no client for - fetch them here
			// (best-effort - a failure just means the export goes out
			// without them, not that export itself should fail) and hand
			// them to the backend to embed as-is.
			const [appGroups, appDisplayOrder] = await Promise.all([
				this.$api.users.getCustomStorage('app_groups').then(res => res.data.data.data).catch(() => undefined),
				this.$api.users.getCustomStorage('app_display_order').then(res => res.data.data.data).catch(() => undefined),
			])
			const userCustom = {}
			if (appGroups)
				userCustom.app_groups = appGroups
			if (appDisplayOrder)
				userCustom.app_display_order = appDisplayOrder

			window.location.href = this.$api.backup.getExportUrl(excluded, userCustom)
		},

		pickImportFile() {
			this.$refs.importFileInput.click()
		},

		onFileSelected(event) {
			const file = event.target.files && event.target.files[0]
			event.target.value = ''
			if (!file)
				return

			this.$buefy.dialog.confirm({
				title: this.$t('Import from backup'),
				message: this.$t('This uploads the archive and shows you what\'s inside - ports, volumes, and any name/port conflicts - before anything installs. Nothing happens on this server until you confirm on the next screen.'),
				confirmText: this.$t('Upload'),
				cancelText: this.$t('Cancel'),
				type: 'is-dark',
				onConfirm: () => this.startImportPreview(file),
			})
		},

		startImportPreview(file) {
			this.result = null
			this.error = ''
			this.preview = null
			this.uploading = true
			this.uploadPercent = 0
			this.previewing = false

			this.$api.backup.importPreview(file, (progressEvent) => {
				if (!progressEvent.total)
					return
				this.uploadPercent = Math.round((progressEvent.loaded / progressEvent.total) * 100)
				if (this.uploadPercent >= 100) {
					this.uploading = false
					this.previewing = true
				}
			}).then((res) => {
				const data = res.data.data
				this.preview = {
					previewId: data.preview_id,
					apps: (data.apps || []).map(app => ({
						...app,
						skip: app.name_conflict,
						services: (app.services || []).map(svc => ({ ...svc, envOpen: false })),
					})),
				}
			}).catch((err) => {
				this.error = err.response?.data?.message || this.$t('Could not read that archive.')
			}).finally(() => {
				this.uploading = false
				this.previewing = false
			})
		},

		openFolderPicker(volume) {
			this.$buefy.modal.open({
				parent: this,
				component: FolderPickerModal,
				props: { initialPath: volume.source },
				hasModalCard: true,
				trapFocus: true,
				canCancel: ['escape', 'outside'],
				scroll: 'keep',
				animation: 'zoom-in',
				events: {
					select: (path) => {
						volume.source = path
					},
				},
			})
		},

		confirmImport() {
			const apps = this.preview.apps.map(app => ({
				name: app.name,
				skip: app.skip,
				ports: app.services.flatMap(svc => svc.ports.map(p => ({
					service_name: svc.service_name,
					target: p.target,
					protocol: p.protocol,
					published: p.published,
				}))),
				volumes: app.services.flatMap(svc => svc.volumes.map(v => ({
					service_name: svc.service_name,
					target: v.target,
					source: v.source,
				}))),
				env: app.services.flatMap(svc => svc.env.map(e => ({
					service_name: svc.service_name,
					key: e.key,
					value: e.value,
				}))),
			}))

			this.confirming = true
			this.error = ''
			this.$api.backup.importConfirm(this.preview.previewId, apps).then(async (res) => {
				this.result = res.data.data
				this.preview = null
				await this.applyImportedUserCustom(this.result.user_custom)
			}).catch((err) => {
				this.error = err.response?.data?.message || this.$t('Import failed.')
			}).finally(() => {
				this.confirming = false
			})
		},

		// Folder groupings and dashboard order came back verbatim from the
		// archive (AppManagement has no client for the service that owns
		// them, so it never touched their contents) - merge them in here,
		// add-only: an existing folder with the same id is left alone
		// rather than overwritten, and an imported folder only keeps the
		// app names that actually made it into "Imported" this run, so a
		// skipped/failed app never shows up in a folder with nothing behind
		// it.
		async applyImportedUserCustom(userCustom) {
			if (!userCustom)
				return
			const importedNames = new Set(this.result.imported || [])

			if (userCustom.app_groups && userCustom.app_groups.length) {
				const existing = await this.$api.users.getCustomStorage('app_groups')
					.then(res => res.data.data.data || []).catch(() => [])
				const existingIds = new Set(existing.map(g => g.id))
				const merged = existing.slice()
				for (const group of userCustom.app_groups) {
					if (existingIds.has(group.id))
						continue
					const appNames = (group.appNames || []).filter(name => importedNames.has(name))
					if (!appNames.length)
						continue
					merged.push({ ...group, appNames })
				}
				if (merged.length !== existing.length)
					await this.$api.users.setCustomStorage('app_groups', { data: merged })
			}

			if (userCustom.app_display_order && userCustom.app_display_order.length) {
				const existing = await this.$api.users.getCustomStorage('app_display_order')
					.then(res => res.data.data.data || []).catch(() => [])
				const existingSet = new Set(existing)
				const additions = userCustom.app_display_order.filter(name => importedNames.has(name) && !existingSet.has(name))
				if (additions.length)
					await this.$api.users.setCustomStorage('app_display_order', { data: [...existing, ...additions] })
			}
		},
	},
}
</script>

<template>
	<div class="modal-card backup-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Backup & Restore') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Move every app and its data to a different server in one step.') }}
			</p>

			<!-- Export Start -->
			<div class="action-card mb-4 p-3">
				<p class="has-text-weight-semibold mb-2">
					{{ $t('Export') }}
				</p>
				<p class="mb-3 has-text-grey is-size-7">
					{{ $t('Every app\'s compose config is always included. Uncheck an app below to leave its data out - useful if you\'re transferring that data some other way.') }}
				</p>

				<b-loading v-model="exportAppsLoading" :is-full-page="false" />

				<div v-if="!exportAppsLoading" class="export-app-list mb-3">
					<div v-for="app in exportApps" :key="app.name" class="is-flex is-align-items-center py-1">
						<b-checkbox
							:value="!exportExcluded[app.name]" size="is-small" class="is-flex-grow-1"
							:disabled="!app.data_paths || app.data_paths.length === 0"
							@input="(checked) => $set(exportExcluded, app.name, !checked)"
						>
							{{ app.display_name || app.name }}
						</b-checkbox>
						<span class="is-size-7 has-text-grey">{{ humanSize(app.data_size_bytes) }}</span>
					</div>
				</div>

				<b-button rounded size="is-small" type="is-dark" @click="confirmExport">
					{{ $t('Export everything') }}
				</b-button>
			</div>
			<!-- Export End -->

			<!-- Import Start -->
			<div class="action-card p-3">
				<p class="has-text-weight-semibold mb-2">
					{{ $t('Import') }}
				</p>
				<p v-if="!preview" class="mb-3 has-text-grey is-size-7">
					{{ $t('Upload an exported archive to review its apps - ports, volumes, and conflicts - before anything installs.') }}
				</p>

				<template v-if="!preview">
					<input ref="importFileInput" type="file" accept=".gz,.tar.gz" class="is-hidden" @change="onFileSelected">
					<b-button
						rounded size="is-small" type="is-dark"
						:disabled="uploading || previewing" @click="pickImportFile"
					>
						{{ $t('Choose backup file…') }}
					</b-button>

					<div v-if="uploading" class="mt-3">
						<p class="is-size-7 mb-1">
							{{ $t('Uploading…') }} {{ uploadPercent }}%
						</p>
						<progress class="progress is-small is-dark" :value="uploadPercent" max="100" />
					</div>
					<div v-if="previewing" class="mt-3">
						<p class="is-size-7">
							{{ $t('Reading the archive…') }}
						</p>
						<progress class="progress is-small is-dark" />
					</div>
				</template>

				<!-- Review screen Start -->
				<template v-else>
					<div v-for="app in preview.apps" :key="app.name" class="import-app-card mb-3 p-2">
						<div class="is-flex is-align-items-center mb-1">
							<b-checkbox
								v-model="app.skip" size="is-small" class="is-flex-grow-1"
								:disabled="app.name_conflict" :true-value="false" :false-value="true"
							>
								{{ app.display_name || app.name }}
							</b-checkbox>
							<span v-if="app.name_conflict" class="tag is-warning is-size-7">
								{{ $t('Already exists on this server - will be skipped') }}
							</span>
						</div>

						<template v-if="!app.skip">
							<div v-for="svc in app.services" :key="svc.service_name" class="ml-4">
								<div v-for="port in svc.ports" :key="`${svc.service_name}-${port.target}-${port.protocol}`" class="is-flex is-align-items-center mb-1">
									<span class="is-size-7 has-text-grey port-label">{{ port.target }}/{{ port.protocol }}</span>
									<b-input
										v-model="port.published" size="is-small" class="ml-2"
										:placeholder="$t('Host port')"
									/>
									<span v-if="port.conflict" class="tag is-danger is-size-7 ml-2">
										{{ $t('In use - pick a different port') }}
									</span>
								</div>
								<div v-for="vol in svc.volumes" :key="`${svc.service_name}-${vol.target}`" class="is-flex is-align-items-center mb-1">
									<span class="is-size-7 has-text-grey volume-label" :title="vol.target">{{ vol.target }}</span>
									<b-input v-model="vol.source" size="is-small" class="ml-2 is-flex-grow-1" />
									<b-button size="is-small" class="ml-2" @click="openFolderPicker(vol)">
										{{ $t('Browse…') }}
									</b-button>
								</div>

								<template v-if="svc.env && svc.env.length">
									<p class="is-size-7 has-text-grey is-clickable mt-1 mb-1" @click="svc.envOpen = !svc.envOpen">
										<b-icon :icon="svc.envOpen ? 'down-outline' : 'right-outline'" pack="casa" size="is-small" />
										{{ $t('Environment Variables') }} ({{ svc.env.length }})
									</p>
									<div v-if="svc.envOpen" class="ml-4">
										<div v-for="e in svc.env" :key="`${svc.service_name}-${e.key}`" class="is-flex is-align-items-center mb-1">
											<span class="is-size-7 has-text-grey env-label" :title="e.key">{{ e.key }}</span>
											<b-input v-model="e.value" size="is-small" class="ml-2 is-flex-grow-1" />
										</div>
									</div>
								</template>
							</div>
						</template>
					</div>

					<b-button rounded size="is-small" type="is-dark" :loading="confirming" @click="confirmImport">
						{{ $t('Confirm Import') }}
					</b-button>
				</template>
				<!-- Review screen End -->

				<p v-if="error" class="mt-3 has-text-danger is-size-7">
					{{ error }}
				</p>

				<div v-if="result" class="mt-3">
					<p v-if="result.imported && result.imported.length" class="is-size-7 has-text-success mb-1">
						{{ $t('Imported') }}: {{ result.imported.join(', ') }}
					</p>
					<template v-if="result.skipped && result.skipped.length">
						<p class="is-size-7 has-text-weight-semibold mb-1">
							{{ $t('Skipped') }}
						</p>
						<p v-for="app in result.skipped" :key="app.name" class="is-size-7 has-text-grey mb-1">
							{{ app.name }} — {{ app.reason }}
						</p>
					</template>
					<template v-if="result.failed && result.failed.length">
						<p class="is-size-7 has-text-weight-semibold has-text-danger mb-1">
							{{ $t('Failed') }}
						</p>
						<p v-for="app in result.failed" :key="app.name" class="is-size-7 has-text-danger mb-1">
							{{ app.name }} — {{ app.reason }}
						</p>
					</template>
				</div>
			</div>
			<!-- Import End -->
		</section>
	</div>
</template>

<style lang="scss" scoped>
.backup-modal {
	.modal-card-body {
		min-height: 8rem;
	}

	.action-card {
		border: 1px solid $border;
		border-radius: 8px;
	}

	.export-app-list {
		max-height: 12rem;
		overflow-y: auto;
	}

	.import-app-card {
		border: 1px solid $border;
		border-radius: 8px;
	}

	.port-label {
		min-width: 5rem;
	}

	.volume-label {
		min-width: 8rem;
		max-width: 8rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.env-label {
		min-width: 8rem;
		max-width: 8rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
}
</style>
