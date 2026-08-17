import { api } from "./service.js";

const PREFIX = "/sys"

const sys = {

	// Get websocket port
	getSocketPort() {
		return api.get(`${PREFIX}/socket-port`);
	},

	// Check if need init
	guideCheck() {
		return api.get(`${PREFIX}/state`);
	},

	// check system version
	getVersion() {
		return api.get(`${PREFIX}/version`);
	},

	// Hardware Info
	hardwareInfo() {
		return api.get(`${PREFIX}/hardware`)
	},

	// get cpu info
	getCpuInfo() {
		return api.get(`${PREFIX}/cpu`);
	},

	// get disk info
	getDiskInfo() {
		return api.get(`${PREFIX}/disk`);
	},

	// usage for every mounted filesystem (like `df -h`)
	getAllDisksUsage() {
		return api.get(`${PREFIX}/disks-usage`);
	},

	// active connections grouped by remote IP - who's currently connected
	getConnections() {
		return api.get(`${PREFIX}/connections`);
	},

	// get memory info
	getMemoryInfo() {
		return api.get(`${PREFIX}/mem`);
	},

	// get network info
	getNetworkInfo() {
		return api.get(`${PREFIX}/network`);
	},

	// get logs
	getLogs() {
		return api.get(`${PREFIX}/logs`);
	},

	//Get Debug Info
	getDebugInfo() {
		return api.get(`${PREFIX}/debug`);
	},

	// get system utilization
	getUtilization() {
		return api.get(`${PREFIX}/utilization`);
	},

	// proxy request
	getProxyRequestContent(url) {
		return api.get(`${PREFIX}/proxy?url=${url}`)
	},

	// get casaos server port
	getServerPort() {
		return api.get(`/gateway/port`);
	},

	// edit casaos server port
	editServerPort(data) {
		return api.put(`/gateway/port`, data);
	},

	// get usb status
	getUsbStatus() {
		return api.get(`/usb/usb-auto-mount`);
	},

	// Toggle usb auto-mount
	toggleUsbAutoMount(data) {
		return api.put(`/usb/usb-auto-mount`, data);
	},

	// update CasaOS
	updateCasaOS() {
		return api.post(`${PREFIX}/update`);
	},

	// update from this fork's own repo (github.com/cvd-unmatched/casa-os)
	updateFromRepo() {
		return api.post(`${PREFIX}/update-fork`);
	},

	// check whether a newer release of this fork is available
	checkForkUpdate() {
		return api.get(`${PREFIX}/update-fork/check`);
	},

	// upload a custom app icon to the configured icon storage disk.
	// The axios instance (service.js) sets a hard default
	// Content-Type: application/json on every request, which a FormData
	// body doesn't automatically override - explicitly clearing it here
	// (not setting it to a string) is what lets the browser generate the
	// correct multipart/form-data header with its boundary itself.
	uploadCustomIcon(formData) {
		return api.post(`${PREFIX}/custom-icon`, formData, {
			headers: { 'Content-Type': undefined },
		});
	},

	// have the server download a remote icon url and save it as a local
	// resized WebP on the given disk - used by the bulk "convert all icons
	// to local WebP" button, so apps can be migrated without the browser
	// having to fetch and re-upload every icon itself.
	convertIconFromUrl(mountpoint, url) {
		return api.post(`${PREFIX}/custom-icon-from-url`, { mountpoint, url });
	},

	// webhook notification destinations (container crashes, image updates,
	// disk warnings, package delivery status)
	getWebhooks() {
		return api.get(`${PREFIX}/webhooks`);
	},

	setWebhooks(config) {
		return api.post(`${PREFIX}/webhooks`, config);
	},

	testWebhook(type, url) {
		return api.post(`${PREFIX}/webhooks/test`, { type, url });
	},

	// stop casaos
	stopCasaOS() {
		return api.post(`${PREFIX}/stop`);
	},

	//Check web ui Port
	checkUiPort(url) {
		return api.get(url);
	},

	// Get system apps
	getSystemApps() {
		return api.get(`${PREFIX}/apps-state`)
	},

	// Check ssh login
	checkSshLogin(data) {
		return api.post(`${PREFIX}/ssh-login`, data);
	},

	// power -- data:shutdown
	// power -- data:restart
	power(data) {
		return api.put(`${PREFIX}/state/${data}`);
	},
}
export default sys;
