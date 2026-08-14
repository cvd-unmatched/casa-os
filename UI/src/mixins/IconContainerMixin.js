import has from 'lodash/has'
import { hasVideoThumbType, hasHeicThumbType } from '@/mixins/mixin'
import { convertHeicUrl, isHeic } from '@/utils/heic'

export default {
	props: {
		item: {}
	},
	data() {
		return {
			isLoaded: false,
			imageData: "",
			isWide: true,
			io: {},
			inViewPort: false
		}
	},
	computed: {
		showThumb() {
			return (this.isLoaded && this.hasThumb(this.item)) || this.isSvg
		},
		isShared() {
			const extensions = this.item.extensions
			if (extensions === null) {
				return false
			} else {
				if (has(extensions, 'share')) {
					return extensions.share.shared === "true"
				} else {
					return false
				}
			}
		},
		isSvg() {
			return this.getFileExt(this.item) === 'svg'
		},
		isVideoThumb() {
			return hasVideoThumbType.indexOf(this.getFileExt(this.item).toLowerCase()) > -1
		},
		isHeicThumb() {
			return isHeic(this.getFileExt(this.item))
		},
	},
	watch: {
		inViewPort(value) {
			if (value) {
				this.loadImage();
			}
		}
	},
	created() {
		this.io = new IntersectionObserver((events) => {
			const { target, isIntersecting } = events[0]
			if (isIntersecting && !this.inViewPort) {
				this.inViewPort = true
				this.io.unobserve(target)
			}
		})
	},
	mounted() {
		if (this.hasThumb(this.item)) {
			this.io.observe(this.$el);
		}
	},
	methods: {
		loadImage() {
			if (this.isSvg) {
				this.imageData = this.getFileUrl(this.item)
				return
			}
			if (this.isVideoThumb) {
				this.loadVideoFrame()
				return
			}
			if (this.isHeicThumb) {
				this.loadHeicImage()
				return
			}

			const imgUrl = this.getThumbUrl(this.item)
			let img = new Image();
			img.crossOrigin = location.host;
			img.src = imgUrl;
			img.onload = () => {
				this.drawToCanvas(img, img.width, img.height)
			};
			img.onerror = (e, s) => {
				console.log(e, s);
			}
		},

		// heic2any needs the actual file, not the (image-only) thumbnail
		// endpoint - fetch and convert it, then draw the result the same way
		// a normal image thumbnail is drawn.
		async loadHeicImage() {
			try {
				const objectUrl = await convertHeicUrl(this.getFileUrl(this.item))
				const img = new Image();
				img.src = objectUrl;
				img.onload = () => {
					this.drawToCanvas(img, img.width, img.height)
				};
				img.onerror = (e) => console.log(e)
			}
			catch (error) {
				console.log(error)
			}
		},

		// Grabs a single frame from the video as its "thumbnail" - a hidden
		// <video> element loads the actual file, seeks a little way in (the
		// very first frame is often just black/a fade-in), then the current
		// frame is drawn to canvas the same way an image thumbnail is.
		loadVideoFrame() {
			const video = document.createElement('video')
			video.crossOrigin = location.host
			video.preload = 'metadata'
			video.muted = true
			video.src = this.getFileUrl(this.item)

			video.onloadedmetadata = () => {
				video.currentTime = Math.min(1, video.duration / 10 || 0)
			}
			video.onseeked = () => {
				this.drawToCanvas(video, video.videoWidth, video.videoHeight)
			}
			video.onerror = (e) => console.log(e)
		},

		drawToCanvas(source, width, height) {
			if (!width || !height) return
			const canvas = document.createElement('canvas');
			canvas.width = width;
			canvas.height = height;
			const ctx = canvas.getContext('2d');
			ctx.drawImage(source, 0, 0, width, height);
			this.isWide = width > height
			this.isLoaded = true
			this.imageData = canvas.toDataURL('image/png');
		},
	},
}
