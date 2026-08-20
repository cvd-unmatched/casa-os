<script>
export default {
	name: 'BackupModal',
	data() {
		return {
			uploading: false,
			uploadPercent: 0,
			installing: false,
			result: null,
			error: '',
		}
	},
	methods: {
		confirmExport() {
			this.$buefy.dialog.confirm({
				title: this.$t('Export everything'),
				message: this.$t('This downloads every installed app\'s compose config and data as one archive. It can be very large and take a while depending on how much app data you have. If CasaOS is behind a reverse proxy, a long idle transfer may be cut off by the proxy rather than by CasaOS itself.'),
				confirmText: this.$t('Export'),
				cancelText: this.$t('Cancel'),
				type: 'is-dark',
				onConfirm: () => this.startExport(),
			})
		},

		startExport() {
			window.location.href = this.$api.backup.getExportUrl()
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
				message: this.$t('This installs every app in the archive that doesn\'t already exist on this server by name - existing apps are left untouched, never overwritten. This can take several minutes once the upload finishes. Continue?'),
				confirmText: this.$t('Import'),
				cancelText: this.$t('Cancel'),
				type: 'is-dark',
				onConfirm: () => this.startImport(file),
			})
		},

		startImport(file) {
			this.result = null
			this.error = ''
			this.uploading = true
			this.uploadPercent = 0
			this.installing = false

			this.$api.backup.importBackup(file, (progressEvent) => {
				if (!progressEvent.total)
					return
				this.uploadPercent = Math.round((progressEvent.loaded / progressEvent.total) * 100)
				if (this.uploadPercent >= 100) {
					this.uploading = false
					this.installing = true
				}
			}).then((res) => {
				this.result = res.data.data
			}).catch((err) => {
				this.error = err.response?.data?.message || this.$t('Import failed.')
			}).finally(() => {
				this.uploading = false
				this.installing = false
			})
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

			<div class="action-card mb-4 p-3">
				<p class="has-text-weight-semibold mb-2">
					{{ $t('Export') }}
				</p>
				<p class="mb-3 has-text-grey is-size-7">
					{{ $t('Downloads a single archive with every app\'s compose config and data.') }}
				</p>
				<b-button rounded size="is-small" type="is-dark" @click="confirmExport">
					{{ $t('Export everything') }}
				</b-button>
			</div>

			<div class="action-card p-3">
				<p class="has-text-weight-semibold mb-2">
					{{ $t('Import') }}
				</p>
				<p class="mb-3 has-text-grey is-size-7">
					{{ $t('Restores apps from an exported archive. Apps that already exist on this server (by name) are skipped, never overwritten.') }}
				</p>

				<input ref="importFileInput" type="file" accept=".gz,.tar.gz" class="is-hidden" @change="onFileSelected">
				<b-button
					rounded size="is-small" type="is-dark"
					:disabled="uploading || installing" @click="pickImportFile"
				>
					{{ $t('Choose backup file…') }}
				</b-button>

				<div v-if="uploading" class="mt-3">
					<p class="is-size-7 mb-1">
						{{ $t('Uploading…') }} {{ uploadPercent }}%
					</p>
					<progress class="progress is-small is-dark" :value="uploadPercent" max="100" />
				</div>

				<div v-if="installing" class="mt-3">
					<p class="is-size-7">
						{{ $t('Installing on the server - this can take several minutes. Don\'t close this tab.') }}
					</p>
					<progress class="progress is-small is-dark" />
				</div>

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
}
</style>
