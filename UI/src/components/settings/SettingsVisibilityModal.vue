<script>
const hiddenSettingsConfig = 'hidden_settings'

export const SETTINGS_CATALOG = [
	{ key: 'search_bar_toggle', label: 'Show Search Bar' },
	{ key: 'search_engine', label: 'Search Engine' },
	{ key: 'language', label: 'Language' },
	{ key: 'webui_port', label: 'WebUI Port' },
	{ key: 'wallpaper', label: 'Wallpaper' },
	{ key: 'icon_storage', label: 'Icon Storage Disk' },
	{ key: 'convert_icons', label: 'Convert all icons to local WebP' },
	{ key: 'github', label: 'GitHub' },
	{ key: 'show_other_docker', label: 'Show other Docker container app(s)' },
	{ key: 'news_feed', label: 'Show news feed from CasaOS Blog' },
	{ key: 'recommended_apps', label: 'Show Recommended Apps' },
	{ key: 'usb_automount', label: 'Automount USB Drive' },
]

export { hiddenSettingsConfig }

export default {
	name: 'SettingsVisibilityModal',
	props: {
		hiddenKeys: {
			type: Array,
			default: () => [],
		},
	},
	data() {
		return {
			catalog: SETTINGS_CATALOG,
			hidden: [...this.hiddenKeys],
		}
	},
	methods: {
		isChecked(key) {
			return this.hidden.indexOf(key) === -1
		},
		toggle(key) {
			const index = this.hidden.indexOf(key)
			if (index === -1) this.hidden.push(key)
			else this.hidden.splice(index, 1)
			this.$emit('change', this.hidden)
		},
	},
}
</script>

<template>
	<div class="modal-card settings-visibility-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Choose settings to show') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Hide settings rows you never use - they stay off until you turn them back on here.') }}
			</p>
			<label
				v-for="item in catalog" :key="item.key"
				class="setting-option is-flex is-align-items-center mb-2 p-3"
			>
				<b-checkbox :value="isChecked(item.key)" class="mr-2" @input="toggle(item.key)" />
				<span class="is-flex-grow-1">{{ $t(item.label) }}</span>
			</label>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.settings-visibility-modal {
	.modal-card-body {
		min-height: 8rem;
	}

	.setting-option {
		border: 1px solid $border;
		border-radius: 8px;
		cursor: pointer;
	}
}
</style>
