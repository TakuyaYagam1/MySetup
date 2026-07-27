local scheme = require("scheme.default")

local M = {
	scheme = scheme,

	terminal = "foot",
	zen = "zen",
	cursor = "cursor",
	vscode = "code",
	zed = "zeditor",
	datagrip = "datagrip",
	fileExplorer = "thunar",
	antigravity = "antigravity",
	amneziaVPN = "AmneziaVPN",
	v2rayN = "v2rayN",
	spotify = "spotify",
	vesktop = "vesktop",
	telegram = "Telegram",
	btop = "foot --app-id btop --title btop btop",

	touchpadDisableTyping = true,
	touchpadScrollFactor = 0.3,
	workspaceSwipeFingers = 3,
	gestureFingers = 4,
	gestureFingersMore = 5,

	blurEnabled = true,
	blurSpecialWs = false,
	blurPopups = true,
	blurInputMethods = true,
	blurSize = 8,
	blurPasses = 2,
	blurXray = false,

	shadowEnabled = true,
	shadowRange = 20,
	shadowRenderPower = 3,

	workspaceGaps = 20,
	windowGapsIn = 10,
	windowGapsOut = 40,
	singleWindowGapsOut = 20,

	windowOpacity = 0.75,
	footWindowOpacity = 0.85,
	windowRounding = 10,
	windowBorderSize = 3,

	volumeStep = 10,
	cursorTheme = "sweet-cursors",
	cursorSize = 24,

	kbMoveWinToWs = "SUPER + SHIFT",
	kbMoveWinToWsGroup = "CTRL + SUPER + ALT",
	kbGoToWs = "SUPER",
	kbGoToWsGroup = "CTRL + SUPER",

	kbNextWs = "CTRL + SUPER + right",
	kbPrevWs = "CTRL + SUPER + left",
	kbToggleSpecialWs = "SUPER + Tab",

	kbWindowGroupCycleNext = "ALT + Tab",
	kbWindowGroupCyclePrev = "SHIFT + ALT + Tab",
	kbUngroup = "SUPER + U",
	kbToggleGroup = "SUPER + Comma",

	kbMoveWindow = "SUPER + Z",
	kbResizeWindow = "SUPER + X",
	kbWindowPip = "SUPER + ALT + Backslash",
	kbPinWindow = "SUPER + P",
	kbWindowFullscreen = "CTRL + SHIFT + Return",
	kbWindowBorderedFullscreen = "SUPER + SHIFT + Return",
	kbToggleWindowFloating = "SUPER + Space",
	kbCloseWindow = "SUPER + Q",

	kbSystemMonitor = "CTRL + SHIFT + ALT + Escape",
	kbMusic = "SUPER + M",
	kbCommunication = "SUPER + D",
	kbTodo = "SUPER + T",

	kbTerminal = "SUPER + Return",
	kbZen = "SUPER + SHIFT + F",
	kbCursor = "SUPER + SHIFT + C",
	kbVscode = "SUPER + SHIFT + V",
	kbZed = "SUPER + SHIFT + Z",
	kbDatagrip = "SUPER + SHIFT + D",
	kbFileExplorer = "SUPER + E",
	kbAntigravity = "SUPER + SHIFT + A",
	kbAmneziaVPN = "SUPER + SHIFT + Q",
	kbv2rayN = "SUPER + SHIFT + N",
	kbSpotify = "SUPER + SHIFT + I",
	kbTelegram = "SUPER + SHIFT + T",
	kbVesktop = "SUPER + SHIFT + B",
	kbBtop = "CTRL + SHIFT + Escape",

	kbRecord = "SUPER + R",
	kbSession = "CTRL + ALT + Delete",
	kbShowSidebar = "SUPER + N",
	kbClearNotifs = "CTRL + ALT + C",
	kbShowPanels = "SUPER + K",
	kbLock = "SUPER + L",
	kbRestoreLock = "SUPER + ALT + L",
}

M.shadowColour = "rgba(" .. scheme.surface .. "d4)"
M.activeWindowBorderColour = "rgba(" .. scheme.primary .. "e6)"
M.inactiveWindowBorderColour = "rgba(" .. scheme.onSurfaceVariant .. "11)"

return M
