// Browsers can't render .heic/.heif directly via <img> (no native decode
// support outside Safari), so both the grid thumbnail and the full-size
// viewer need to fetch the raw file and convert it client-side first.
// Converted object URLs are cached per source URL so paging back and forth
// in the viewer, or scrolling a file list, doesn't re-convert repeatedly.
import heic2any from 'heic2any'

const cache = new Map()

/**
 * @description: Fetches a .heic/.heif file and converts it to a displayable
 * JPEG object URL.
 * @param {string} fileUrl
 * @return {Promise<string>}
 */
export async function convertHeicUrl(fileUrl) {
	if (cache.has(fileUrl))
		return cache.get(fileUrl)

	const promise = (async () => {
		const sourceBlob = await fetch(fileUrl).then(res => res.blob())
		const converted = await heic2any({ blob: sourceBlob, toType: 'image/jpeg', quality: 0.85 })
		const resultBlob = Array.isArray(converted) ? converted[0] : converted
		return URL.createObjectURL(resultBlob)
	})()

	cache.set(fileUrl, promise)
	return promise
}

export function isHeic(ext) {
	const lower = (ext || '').toLowerCase()
	return lower === 'heic' || lower === 'heif'
}
