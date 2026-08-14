<template>
	<div class="modal-card">
		<!-- Modal-Card Header Start -->
		<header class="modal-card-head">
			<div class="is-flex-grow-1">
				<h3 class="title is-header">{{ $t('Change wallpaper') }}</h3>
			</div>

		</header>
		<!-- Modal-Card Header End -->
		<!-- Modal-Card Body Start -->
		<section class="modal-card-body ">
			<div class="preview-card">
				<div :style="backgroundStyleObj" class="preview-image">
					<b-image :src="require('@/assets/background/preview-widget.svg')" class="preview-widget"></b-image>
					<b-image :src="require('@/assets/background/blank.png')"></b-image>
				</div>
			</div>

			<div class="columns mt-5 mb-5 is-variable is-2">
				<div v-for="(item, index) in wallpaperItems" :key="'wallpaper' + index" class="column">
					<div :class="{ active: checkActive(item.path) }" class="image-list-item is-clickable"
						@click="changeWallpaper(item.path)">
						<b-image :src="item.path"></b-image>
					</div>
				</div>

				<div class="column is-relative is-one-quarter">
					<div :class="{ active: checkActiveFrom('Upload') }" class="upload-button-container is-clickable">
						<div id="upload-wallpaper"
							class="upload-button is-flex is-align-items-center is-justify-content-center ">
							<b-icon icon="picture-upload-outline" pack="casa" size="is-large"></b-icon>

						</div>
						<b-loading v-model="isUpLoading" :can-cancel="false" :is-full-page="false"></b-loading>
					</div>

				</div>
			</div>

			<div class="color-picker is-flex is-align-items-center">
				<span class="mr-3 has-text-grey is-size-7">{{ $t('Solid color') }}</span>
				<span
					v-for="color in backgroundColors" :key="color" class="color-swatch mr-2"
					:class="{ active: from === 'Color' && path === color }" :style="{ background: color }"
					@click="changeColor(color)"
				/>
				<label
					class="color-swatch custom-swatch" :class="{ active: from === 'Color' && !isPresetColor }"
					:style="customColorSwatchStyle" :title="$t('Custom color')"
				>
					<input type="color" class="color-input" :value="from === 'Color' ? path : '#909599'" @input="changeColor($event.target.value)">
				</label>
			</div>
		</section>
		<!-- Modal-Card Body End -->
		<!-- Modal-Card Footer Start-->
		<footer class="modal-card-foot is-flex is-align-items-center">
			<div class="is-flex-grow-1"></div>
			<div>
				<b-button :label="$t('Cancel')" rounded @click="$emit('close')" />
				<b-button :label="$t('Apply')" :loading="isLoading" expaned rounded type="is-primary" @click="saveChange" />
			</div>
		</footer>
		<!-- Modal-Card Footer End -->
	</div>
</template>

<script>
const wallpaperConfig = "wallpaper"
import Uploader from 'simple-uploader.js'

const BACKGROUND_COLORS = ['#1E1E2E', '#2D2D3A', '#3D3D50', '#4A4A5E', '#5B8DEF', '#61C454', '#F2994A', '#EB5757', '#9B51E0', '#2D9CDB', '#F2C94C', '#FFFFFF']

export default {
	data() {
		return {
			isLoading: false,
			isUpLoading: false,
			uploader: null,
			backgroundColors: BACKGROUND_COLORS,
			attributes: {
				accept: 'image/png, image/jpeg, image/svg+xml, image/bmp, image/png, image/gif'
			},
			wallpaperItems: [
				{
					name: "Built-in wallpaper 1",
					path: require('@/assets/background/wallpaper01.jpg')
				},
				{
					name: "Built-in wallpaper 2",
					path: require('@/assets/background/wallpaper02.jpg')
				}
			],
			backgroundStyleObj: this.$store.state.wallpaperObject.from === 'Color'
				? { backgroundColor: this.$store.state.wallpaperObject.path }
				: { backgroundImage: `url(${this.parseUrl(this.$store.state.wallpaperObject.path)})` },
			path: this.$store.state.wallpaperObject.path,
			from: this.$store.state.wallpaperObject.from
		}
	},
	components: {},
	computed: {
		isPresetColor() {
			return this.backgroundColors.includes(this.path)
		},
		customColorSwatchStyle() {
			return this.from === 'Color' && !this.isPresetColor
				? { background: this.path }
				: {}
		},
	},
	created() {
		this.uploader = new Uploader({
			target: this.getTargetUrl(),
			singleFile: true,
			testChunks: false,
			uploadMethod: "POST",
			allowDuplicateUploads: true,
			chunkSize: 1024 * 1024 * 1024 * 1024
		});

	},
	mounted() {
		this.uploader.assignBrowse(document.getElementById('upload-wallpaper'), false, true, this.attributes)
		this.uploader.on('filesSubmitted', () => {
			this.isUpLoading = true
			this.$api.sys.getVersion().then(res => {
				this.uploader.opts.headers.Authorization = this.$store.state.access_token || localStorage.getItem("access_token")
				this.uploader.upload()
			})
		})
		this.uploader.on('fileError', () => {
			this.isUpLoading = false
			this.$buefy.toast.open({
				message: this.$t('Upload failed, please try again!'),
				type: 'is-danger'
			})
		})
		this.uploader.on('fileSuccess', (rootFile, file, message) => {
			this.isUpLoading = false
			const res = JSON.parse(message)

			if (res.success === 200) {
				const uploadPath = "SERVER_URL" + res.data.online_path + "&time=" + new Date().getTime()
				this.backgroundStyleObj = { backgroundImage: `url(${this.parseUrl(uploadPath)})` }
				this.path = uploadPath
				this.from = "Upload"

			} else {
				this.$buefy.toast.open({
					message: res.message,
					type: 'is-danger'
				})
			}
		})

	},
	methods: {
		saveChange() {
			let data = {
				path: this.path,
				from: this.from
			}
			this.isLoading = true
			this.$api.users.setCustomStorage(wallpaperConfig, data).then(res => {
				this.isLoading = false
				if (res.data.success === 200) {
					this.$messageBus('dashboardsetting_wallpaper', res.data.data.path.toString())
					this.$emit("close")
					setTimeout(() => {
						this.$store.commit('SET_WALLPAPER', {
							path: res.data.data.path,
							from: res.data.data.from
						})
					}, 300)

				} else {
					this.$buefy.toast.open({
						message: this.$t('Save failed, please try again!'),
						type: 'is-danger'
					})
				}

			})
		},
		changeWallpaper(path) {
			this.backgroundStyleObj = { backgroundImage: `url(${this.parseUrl(path)})` }
			this.path = path
			this.from = "Built-in"
		},

		changeColor(color) {
			this.backgroundStyleObj = { backgroundColor: color }
			this.path = color
			this.from = "Color"
		},

		checkActive(path) {
			return this.path == path
		},
		checkActiveFrom(from) {
			return this.from == from
		},
		getTargetUrl() {
			const accessToken = localStorage.getItem("access_token")
			return `${this.$protocol}//${this.$baseURL}/v1/users/current/image/${wallpaperConfig}?token=${accessToken}&type=wallpaper`
		},
		parseUrl(serverUrl) {
			const newUrl = serverUrl.replace('SERVER_URL', `${this.$protocol}//${this.$baseURL}`)
			return newUrl;
		},
	}
}
</script>

<style lang="scss" scoped>
.preview-card {
	margin-left: 2.625rem;
	margin-right: 2.625rem;
	margin-top: 3rem;
	margin-bottom: 3rem;
	border: 6px solid transparent;
	border-radius: 16px;
	background-clip: padding-box, border-box;
	background-origin: padding-box, border-box;
	background-image: linear-gradient(to right, #fff, #fff),
		linear-gradient(108.27deg,
			rgba(57, 60, 64, 1) 1.44%,
			rgba(92, 96, 102, 1) 55.8%,
			rgba(34, 36, 38, 1) 110.95%);
	box-shadow: 0px 16px 24px rgba(115, 120, 128, 0.4);
	overflow: hidden;

	.preview-image {
		position: relative;
		overflow: hidden;

		.preview-widget {
			position: absolute;
			left: 8.3333%;
			top: 14.81%;
			width: 23.96%;
			z-index: 1;
		}
	}
}

.image-list-item,
.upload-button-container {
	width: 100%;
	height: 100%;
	overflow: hidden;
	border-radius: 5px;
	border-color: transparent;
	border-width: 2px;
	border-style: solid;
	transition: border-color 0.3s ease;

	&.active {
		border-color: $primary;
	}

	&:hover {
		border-color: $primary;
	}

	.image {
		border-radius: 4px;
		border: #fff 1px solid;
		overflow: hidden;
		background-size: cover;
	}

	.upload-button {
		width: 100%;
		height: 100%;
		border-radius: 4px;
		border: #fff 1px solid;
		overflow: hidden;
		background-color: rgba(240, 242, 245, 1);
	}
}

.preview-image {
	background-size: cover;
	background-repeat: no-repeat;
	overflow: hidden;
	background-position: center center;
	transition: background-image 0.3s ease;
}

.color-picker {
	margin-left: 2.625rem;
	margin-right: 2.625rem;
	margin-bottom: 1.5rem;

	.color-swatch {
		display: inline-block;
		width: 1.5rem;
		height: 1.5rem;
		border-radius: 50%;
		cursor: pointer;
		box-sizing: border-box;
		border: 2px solid transparent;
		transition: border-color 0.15s;

		&.active {
			border-color: $primary;
		}

		&:hover {
			border-color: $primary;
		}
	}

	.custom-swatch {
		position: relative;
		overflow: hidden;
		background: conic-gradient(red, yellow, lime, cyan, blue, magenta, red);

		.color-input {
			position: absolute;
			top: -25%;
			left: -25%;
			width: 150%;
			height: 150%;
			border: none;
			padding: 0;
			cursor: pointer;
			opacity: 0;
		}
	}
}
</style>
