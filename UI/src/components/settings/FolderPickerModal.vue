<script>
export default {
	name: 'FolderPickerModal',
	props: {
		initialPath: {
			type: String,
			default: '/',
		},
	},
	data() {
		return {
			currentPath: this.initialPath && this.initialPath !== '' ? this.initialPath : '/',
			entries: [],
			isLoading: false,
			error: '',
			newFolderName: '',
			creatingFolder: false,
		}
	},
	computed: {
		breadcrumbSegments() {
			const parts = this.currentPath.split('/').filter(p => p !== '')
			const segments = [{ label: '/', path: '/' }]
			let built = ''
			for (const part of parts) {
				built += `/${part}`
				segments.push({ label: part, path: built })
			}
			return segments
		},
	},
	created() {
		this.load(this.currentPath)
	},
	methods: {
		load(path) {
			this.isLoading = true
			this.error = ''
			this.$api.folder.getList(path).then((res) => {
				if (res.data.success !== 200) {
					this.error = res.data.message || this.$t('Could not list that folder.')
					return
				}
				this.currentPath = path
				this.entries = (res.data.data.content || []).filter(item => item.is_dir)
			}).catch(() => {
				this.error = this.$t('Could not list that folder.')
			}).finally(() => {
				this.isLoading = false
			})
		},

		openEntry(entry) {
			this.load(entry.path)
		},

		createFolder() {
			const name = this.newFolderName.trim()
			if (!name)
				return
			this.creatingFolder = true
			const newPath = this.currentPath === '/' ? `/${name}` : `${this.currentPath}/${name}`
			this.$api.folder.create(newPath).then(() => {
				this.newFolderName = ''
				this.load(this.currentPath)
			}).catch(() => {
				this.$buefy.toast.open({ message: this.$t('Could not create that folder.'), type: 'is-danger' })
			}).finally(() => {
				this.creatingFolder = false
			})
		},

		selectCurrent() {
			this.$emit('select', this.currentPath)
			this.$emit('close')
		},
	},
}
</script>

<template>
	<div class="modal-card folder-picker-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Choose a folder') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<div class="breadcrumb is-size-7 mb-2">
				<span v-for="(segment, i) in breadcrumbSegments" :key="segment.path">
					<a class="is-clickable" @click="load(segment.path)">{{ segment.label }}</a>
					<span v-if="i < breadcrumbSegments.length - 1"> / </span>
				</span>
			</div>

			<b-loading v-model="isLoading" :is-full-page="false" />

			<p v-if="error" class="has-text-danger is-size-7 mb-2">
				{{ error }}
			</p>

			<div v-else-if="!isLoading" class="entry-list mb-3">
				<p v-if="entries.length === 0" class="has-text-grey is-size-7 py-2">
					{{ $t('No subfolders here.') }}
				</p>
				<div
					v-for="entry in entries" :key="entry.path"
					class="entry-row is-flex is-align-items-center is-clickable p-2" @click="openEntry(entry)"
				>
					<b-icon class="mr-2" icon="folder-outline" pack="casa" size="is-small" />
					<span class="one-line">{{ entry.name }}</span>
				</div>
			</div>

			<div class="new-folder is-flex is-align-items-center mb-3">
				<b-input
					v-model="newFolderName" :placeholder="$t('New folder name')"
					size="is-small" class="is-flex-grow-1 mr-2" @keyup.native.enter="createFolder"
				/>
				<b-button
					size="is-small" type="is-dark" rounded :loading="creatingFolder"
					:disabled="!newFolderName.trim()" @click="createFolder"
				>
					{{ $t('Create') }}
				</b-button>
			</div>

			<div class="is-flex is-justify-content-flex-end">
				<b-button size="is-small" rounded class="mr-2" @click="$emit('close')">
					{{ $t('Cancel') }}
				</b-button>
				<b-button size="is-small" type="is-dark" rounded @click="selectCurrent">
					{{ $t('Select this folder') }}
				</b-button>
			</div>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.folder-picker-modal {
	.modal-card-body {
		min-height: 16rem;
		position: relative;
	}

	.entry-list {
		max-height: 14rem;
		overflow-y: auto;
		border: 1px solid $border;
		border-radius: 8px;
	}

	.entry-row:hover {
		background: rgba(0, 0, 0, 0.04);
	}
}
</style>
