<template>
	<div class="mb-5">
		<div class="field is-flex is-align-items-center mb-2">
			<label class="label mb-0 is-flex-grow-1">{{ label }}</label>
			<b-button icon-left="paste-outline" icon-pack="casa" rounded size="is-small" class="mr-2" @click="showPaste = !showPaste">{{ $t('Paste') }}</b-button>
			<b-button  icon-left="plus-outline" icon-pack="casa" rounded size="is-small" @click="addItem">{{ $t('Add') }}</b-button>
		</div>
		<div v-if="showPaste" class="mb-3">
			<b-input v-model="pasteText" type="textarea" :rows="5" :placeholder="$t('Paste a KEY=VALUE list, one per line')"></b-input>
			<div class="is-flex is-justify-content-flex-end mt-2">
				<b-button size="is-small" class="mr-2" @click="cancelPaste">{{ $t('Cancel') }}</b-button>
				<b-button type="is-dark" size="is-small" @click="importPaste">{{ $t('Import') }}</b-button>
			</div>
		</div>
		<div v-if="items.length == 0" class="is-flex is-align-items-center mb-5 info">
			<b-icon icon="warning-solid" size="is-small" pack="casa" class="mr-2 "></b-icon>
			<span>
				{{ message }}
			</span>

		</div>
		<div v-for="(item, index) in items" :key="'port' + index" class="port-item  mr-4">
			<b-icon class="is-clickable" icon="close-outline" pack="casa" size="is-small"
				@click.native="removeItem(index)"></b-icon>
			<template v-if="index < 1">
				<b-field grouped>
					<b-field :label="$t(name1)" expanded>
						<b-input v-model="item.container" :placeholder="$t(name1)" expanded></b-input>
					</b-field>
					<b-field :label="$t(name2)" expanded>
						<b-input v-model="item.host" :placeholder="$t(name2)" expanded></b-input>
					</b-field>

				</b-field>
			</template>
			<template v-else>

				<b-field grouped>
					<b-input v-model="item.container" :placeholder="$t(name1)" expanded></b-input>
					<b-input v-model="item.host" :placeholder="$t(name2)" expanded></b-input>
				</b-field>

			</template>
		</div>

	</div>
</template>

<script>
export default {
	name: 'env-input-group',
	data() {
		return {
			isLoading: false,
			min: 0,
			showPaste: false,
			pasteText: ''
		}
	},
	model: {
		prop: 'vData',
		event: 'change'
	},
	props: {
		vData: Array,
		label: String,
		message: String,
		name1: {
			type: String,
			default: "Key"
		},
		name2: {
			type: String,
			default: "Value"
		},

	},
	computed: {
		items: {
			get() {
				return this.vData
			},
			set(val) {
				this.$emit('change', val)
			}
		}
	},
	methods: {
		addItem() {
			let itemObj = {
				container: "",
				host: ""
			}
			this.items.push(itemObj)
		},

		removeItem(index) {
			this.items.splice(index, 1)
		},

		cancelPaste() {
			this.pasteText = ''
			this.showPaste = false
		},

		importPaste() {
			const parsed = this.pasteText
				.split('\n')
				.map(line => line.trim())
				.filter(line => line.length > 0 && !line.startsWith('#'))
				.map((line) => {
					const withoutExport = line.replace(/^export\s+/, '')
					const eqIndex = withoutExport.indexOf('=')
					if (eqIndex === -1) return null
					const key = withoutExport.slice(0, eqIndex).trim()
					let value = withoutExport.slice(eqIndex + 1).trim()
					if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith('\'') && value.endsWith('\'')))
						value = value.slice(1, -1)
					return key ? { container: key, host: value } : null
				})
				.filter(Boolean)

			if (parsed.length === 0) {
				this.$buefy.toast.open({
					message: this.$t('No KEY=VALUE lines found in that text.'),
					type: 'is-warning'
				})
				return
			}

			const existingKeys = new Set(this.items.map(item => item.container))
			parsed.forEach((item) => {
				if (existingKeys.has(item.container)) {
					// same key pasted again - update the existing row instead of duplicating it
					const existing = this.items.find(i => i.container === item.container)
					existing.host = item.host
				}
				else {
					this.items.push(item)
					existingKeys.add(item.container)
				}
			})

			this.$buefy.toast.open({
				message: this.$t('Imported {count} environment variable(s).', { count: parsed.length }),
				type: 'is-success'
			})
			this.cancelPaste()
		},
	},
}
</script>

