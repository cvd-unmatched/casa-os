// Preset visual themes for folder tiles, as an alternative to a plain solid
// color. Each theme is a gradient background plus a handful of decorations
// (plain emoji - no icon-font/asset pipeline needed, renders consistently
// everywhere) that drift gently via CSS animation in AppFolderCard.vue.
// Adding a new theme is just appending an entry here.
export const FOLDER_THEMES = [
	{ id: 'valentine', label: 'Valentine', gradient: 'linear-gradient(135deg, #ff6b9d 0%, #c9184a 100%)', decorations: ['❤️', '💕', '💖'] },
	{ id: 'autumn', label: 'Autumn', gradient: 'linear-gradient(135deg, #d97706 0%, #7c2d12 100%)', decorations: ['🍁', '🍂', '🍁'] },
	{ id: 'winter', label: 'Winter', gradient: 'linear-gradient(135deg, #38bdf8 0%, #1e3a8a 100%)', decorations: ['❄️', '❄️', '❄️'] },
	{ id: 'music', label: 'Music', gradient: 'linear-gradient(135deg, #7c3aed 0%, #1e1b4b 100%)', decorations: ['🎵', '🎶', '🎵'] },
	{ id: 'games', label: 'Games', gradient: 'linear-gradient(135deg, #14b8a6 0%, #164e63 100%)', decorations: ['🎮', '🕹️', '🎲'] },
	{ id: 'printing', label: 'Printing', gradient: 'linear-gradient(135deg, #64748b 0%, #1e293b 100%)', decorations: ['🖨️', '📄', '🖨️'] },
	{ id: 'party', label: 'Party', gradient: 'linear-gradient(135deg, #f59e0b 0%, #db2777 100%)', decorations: ['🎉', '🎊', '🎈'] },
]

export function findFolderTheme(themeId) {
	return FOLDER_THEMES.find(theme => theme.id === themeId) || null
}
