<script>
export default {
	name: 'HiddenAppsModal',
	props: {
		apps: {
			type: Array,
			default: () => [],
		},
		unhide: {
			type: Function,
			required: true,
		},
	},
	data() {
		return {
			remaining: [...this.apps],
		}
	},
	methods: {
		show(app) {
			this.unhide(app.name)
			this.remaining = this.remaining.filter(item => item.name !== app.name)
			if (this.remaining.length === 0)
				this.$emit('close')
		},
	},
}
</script>

<template>
	<div class="modal-card hidden-apps-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Hidden apps') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<div
				v-for="app in remaining"
				:key="app.name"
				class="hidden-app-row mb-2 is-flex is-align-items-center"
			>
				<b-image
					:src="app.icon"
					:src-fallback="require('@/assets/img/app/default.svg')"
					class="is-32x32 mr-3"
					ratio="1by1"
				/>
				<span class="is-flex-grow-1 one-line" :title="app.title">{{ app.title }}</span>
				<b-button rounded size="is-small" type="is-primary is-light" @click="show(app)">
					{{ $t('Show') }}
				</b-button>
			</div>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.hidden-apps-modal {
	.modal-card-body {
		min-height: 8rem;
		max-height: 24rem;
		overflow-y: auto;
	}

	.hidden-app-row {
		border: 1px solid $border;
		border-radius: 8px;
		padding: 0.5rem 0.75rem;
	}

	.one-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
}
</style>
