package launcher

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/fsnotify/fsnotify"
	"github.com/sqweek/dialog"

	"github.com/shntsk/taktchase/app/beatmap"
	"github.com/shntsk/taktchase/app/beatmap/difficulty"
	"github.com/shntsk/taktchase/app/database"
	"github.com/shntsk/taktchase/app/osuapi"
	"github.com/shntsk/taktchase/app/settings"
	"github.com/shntsk/taktchase/build"
	"github.com/shntsk/taktchase/framework/env"
	"github.com/shntsk/taktchase/framework/platform"
)

type gioPage int

const (
	gioPageHome gioPage = iota
	gioPageLibrary
	gioPageMods
	gioPageSettings
	gioPageProfiles
	gioPageAbout
	gioPageSpeed
)

type gioSettingOption struct {
	value string
	label string
}

type gioSettingField struct {
	key     string
	label   string
	value   reflect.Value
	group   bool
	bool    widget.Bool
	editor  widget.Editor
	click   widget.Clickable
	options []gioSettingOption
}

type gioModItem struct {
	acronym string
	label   string
	group   string
	mod     difficulty.Modifier
}

type gioLauncherUI struct {
	l      *launcher
	window *app.Window
	theme  *material.Theme

	page gioPage

	searchEditor widget.Editor
	mapList      widget.List
	modsList     widget.List
	settingsNavList widget.List
	settingsFieldList widget.List
	profileList  widget.List
	filteredMaps []*beatmap.BeatMap
	lastSearch   string
	mapButtons   map[string]*widget.Clickable

	navButtons     map[gioPage]*widget.Clickable
	modePlay       widget.Clickable
	modeReplay     widget.Clickable
	reloadMaps     widget.Clickable
	openReplay     widget.Clickable
	loadReplay     widget.Clickable
	replayPath     widget.Editor
	startButton    widget.Clickable
	cancelButton   widget.Clickable
	openMods       widget.Clickable
	openSpeed      widget.Clickable
	saveSettings   widget.Clickable
	openProfiles   widget.Clickable
	profileButtons map[string]*widget.Clickable
	profileName    widget.Editor
	createProfile  widget.Clickable
	cloneProfile   widget.Clickable
	deleteProfile  widget.Clickable
	aboutGitHub    widget.Clickable

	modButtons map[difficulty.Modifier]*widget.Clickable

	settingsCategory string
	settingsButtons   map[string]*widget.Clickable
	settingsFields    []*gioSettingField
	settingsBool      map[string]*widget.Bool
	launcherVolume    widget.Editor
	credentialHelp   widget.Clickable
	authorize        widget.Clickable
	resetTokens      widget.Clickable
	endEditor        widget.Editor
	startEditor      widget.Editor
	offsetEditor     widget.Editor
	pitchEditor      widget.Editor
	speedEditor       widget.Editor

	statusMu      sync.RWMutex
	reloadMu      sync.Mutex
	watchMu       sync.Mutex
	refreshTimer  *time.Timer
	status        string
	loading       bool
	loadingPct    float32
	process       *exec.Cmd
	processActive bool
	pendingLatestReplay bool
	latestReplayChecked bool
}

var gioSettingsCategories = []string{
	"General", "Graphics", "Audio", "Input", "Gameplay", "Skin",
	"Cursor", "Objects", "Playfield", "CursorDance", "Knockout",
	"Recording", "Debug", "Credentials", "Launcher",
}

var gioMods = []gioModItem{
	{group: "Difficulty", acronym: "EZ", label: "Easy", mod: difficulty.Easy},
	{group: "Difficulty", acronym: "HR", label: "Hard Rock", mod: difficulty.HardRock},
	{group: "Difficulty", acronym: "HD", label: "Hidden", mod: difficulty.Hidden},
	{group: "Difficulty", acronym: "FL", label: "Flashlight", mod: difficulty.Flashlight},
	{group: "Difficulty", acronym: "MR", label: "Mirror", mod: difficulty.Mirror},
	{group: "Difficulty", acronym: "DA", label: "Difficulty Adjust", mod: difficulty.DifficultyAdjust},
	{group: "Scoring", acronym: "NF", label: "No Fail", mod: difficulty.NoFail},
	{group: "Scoring", acronym: "SD", label: "Sudden Death", mod: difficulty.SuddenDeath},
	{group: "Scoring", acronym: "PF", label: "Perfect", mod: difficulty.Perfect},
	{group: "Scoring", acronym: "V2", label: "ScoreV2", mod: difficulty.ScoreV2},
	{group: "Speed", acronym: "DT", label: "Double Time", mod: difficulty.DoubleTime},
	{group: "Speed", acronym: "NC", label: "Nightcore", mod: difficulty.Nightcore},
	{group: "Speed", acronym: "HT", label: "Half Time", mod: difficulty.HalfTime},
	{group: "Speed", acronym: "DC", label: "Daycore", mod: difficulty.Daycore},
	{group: "Automation", acronym: "RX", label: "Relax", mod: difficulty.Relax},
	{group: "Automation", acronym: "AP", label: "Autopilot", mod: difficulty.Relax2},
	{group: "Automation", acronym: "AT", label: "Autoplay", mod: difficulty.Autoplay},
	{group: "Automation", acronym: "SO", label: "Spun Out", mod: difficulty.SpunOut},
	{group: "Automation", acronym: "TD", label: "Touch Device", mod: difficulty.TouchDevice},
	{group: "Ruleset", acronym: "LZ", label: "Lazer", mod: difficulty.Lazer},
	{group: "Ruleset", acronym: "CL", label: "Classic", mod: difficulty.Classic},
	{group: "Ruleset", acronym: "TC", label: "Traceable", mod: difficulty.Traceable},
}

func StartLauncher() {
	startGioLauncher()
}

func startGioLauncher() {
	platform.StartLogging("launcher")
	loadLauncherConfig()
	settings.CreateDefault()

	l := &launcher{
		bld:    newBuilder(),
		cHold:  make(map[string]*bool),
		winter: false,
	}

	if launcherConfig.Profile == nil {
		profile := "default"
		launcherConfig.Profile = &profile
	}

	defaultPath := filepath.Join(env.ConfigDir(), "default.json")
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		conf := settings.NewConfigFile()
		conf.Graphics.SetDefaults(1920, 1080)
		conf.Save(defaultPath, true)
	}

	l.createConfigList()
	settings.LoadCredentials()

	profile := *launcherConfig.Profile
	conf, err := l.loadConfig(profile)
	if err != nil {
		log.Printf("Gio launcher: failed to load profile %q: %v; using default", profile, err)
		profile = "default"
		conf, err = l.loadConfig(profile)
		if err != nil {
			log.Printf("Gio launcher: failed to load default profile: %v", err)
			conf = settings.NewConfigFile()
			conf.Graphics.SetDefaults(1920, 1080)
			conf.Save(defaultPath, true)
		}
	}

	l.bld.config = profile
	l.currentConfig = conf
	*launcherConfig.Profile = profile

	settings.General.OsuSongsDir = conf.General.OsuSongsDir
	settings.General.OsuSkinsDir = conf.General.OsuSkinsDir
	settings.General.OsuReplaysDir = conf.General.OsuReplaysDir

	ui := newGioLauncherUI(l)

	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("TaktChase"),
			app.Size(unit.Dp(1440), unit.Dp(900)),
		)
		if err := ui.run(window); err != nil {
			log.Println("Gio launcher closed:", err)
		}
		saveLauncherConfig()
		if l.currentConfig != nil {
			l.currentConfig.Save("", false)
		}
		ui.watchMu.Lock()
		if ui.refreshTimer != nil {
			ui.refreshTimer.Stop()
		}
		ui.watchMu.Unlock()
		closeWatcher()
		ui.reloadMu.Lock()
		database.Close()
		ui.reloadMu.Unlock()
		os.Exit(0)
	}()

	app.Main()
}

func newGioLauncherUI(l *launcher) *gioLauncherUI {
	ui := &gioLauncherUI{
		l:                 l,
		page:              gioPageHome,
		mapButtons:        make(map[string]*widget.Clickable),
		navButtons:        make(map[gioPage]*widget.Clickable),
		profileButtons:    make(map[string]*widget.Clickable),
		modButtons:        make(map[difficulty.Modifier]*widget.Clickable),
		settingsButtons:   make(map[string]*widget.Clickable),
		settingsBool:      make(map[string]*widget.Bool),
		settingsCategory:  "General",
		status:              "Preparing beatmap library…",
		loading:             true,
		pendingLatestReplay: launcherConfig.LoadLatestReplay,
	}
	ui.searchEditor.SingleLine = true
	ui.searchEditor.SetText("")
	ui.replayPath.SingleLine = true
	ui.replayPath.SetText(l.bld.replayPath)
	ui.profileName.SingleLine = true
	ui.profileName.SetText("My profile")
	ui.launcherVolume.SingleLine = true
	ui.launcherVolume.SetText(strconv.Itoa(int(launcherConfig.PreviewVolume * 100)))
	ui.speedEditor.SingleLine = true
	ui.pitchEditor.SingleLine = true
	ui.offsetEditor.SingleLine = true
	ui.startEditor.SingleLine = true
	ui.endEditor.SingleLine = true
	ui.speedEditor.SetText(strconv.FormatFloat(l.bld.speed.value, 'f', 2, 64))
	ui.pitchEditor.SetText(strconv.FormatFloat(l.bld.pitch.value, 'f', 2, 64))
	ui.offsetEditor.SetText(strconv.FormatInt(int64(l.bld.offset.value), 10))
	ui.startEditor.SetText(strconv.FormatInt(int64(l.bld.start.value), 10))
	ui.endEditor.SetText(strconv.FormatInt(int64(l.bld.end.value), 10))
	ui.mapList.List.Axis = layout.Vertical
	ui.modsList.List.Axis = layout.Vertical
	ui.settingsNavList.List.Axis = layout.Vertical
	ui.settingsFieldList.List.Axis = layout.Vertical
	ui.profileList.List.Axis = layout.Vertical
	ui.speedEditor.SingleLine = true
	ui.speedEditor.SetText(strconv.FormatFloat(l.bld.speed.value, 'f', 2, 64))
	ui.pitchEditor.SingleLine = true
	ui.pitchEditor.SetText(strconv.FormatFloat(l.bld.pitch.value, 'f', 2, 64))

	for _, page := range []gioPage{gioPageHome, gioPageLibrary, gioPageMods, gioPageSettings, gioPageProfiles, gioPageAbout} {
		ui.navButtons[page] = new(widget.Clickable)
	}
	for _, mod := range gioMods {
		ui.modButtons[mod.mod] = new(widget.Clickable)
	}
	for _, category := range gioSettingsCategories {
		ui.settingsButtons[category] = new(widget.Clickable)
	}

	ui.rebuildSettingsFields()
	return ui
}

func (ui *gioLauncherUI) run(window *app.Window) error {
	ui.window = window
	ui.theme = material.NewTheme()
	ui.theme.Palette = material.Palette{
		Bg:         color.NRGBA{R: 17, G: 20, B: 29, A: 255},
		Fg:         color.NRGBA{R: 235, G: 238, B: 247, A: 255},
		ContrastBg: color.NRGBA{R: 126, G: 111, B: 255, A: 255},
		ContrastFg: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}
	ui.theme.TextSize = unit.Sp(14)

	ui.loadMaps()

	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			saveLauncherConfig()
			if ui.l.currentConfig != nil {
				ui.l.currentConfig.Save("", false)
			}
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			ui.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

func (ui *gioLauncherUI) loadMaps() {
	ui.setLoadState(true, "Scanning beatmaps…", 0)
	go func() {
		ui.reloadMu.Lock()
		defer ui.reloadMu.Unlock()
		database.Close()
		if err := database.Init(); err != nil {
			ui.l.beatmaps = nil
			ui.filteredMaps = nil
			ui.setLoadState(false, "Could not open the beatmap database: "+err.Error(), 0)
			return
		}

		maps := database.LoadBeatmaps(launcherConfig.SkipMapUpdate, func(stage database.ImportStage, processed, target int) {
			message := "Loading beatmaps…"
			progress := float32(0)
			switch stage {
			case database.Discovery:
				message = "Discovering .osu files…"
			case database.Comparison:
				message = "Comparing songs with the library…"
			case database.Cleanup:
				message = "Cleaning outdated entries…"
			case database.Import:
				message = "Importing beatmaps…"
				if target > 0 {
					progress = float32(processed) / float32(target)
				}
			case database.Finished:
				message = "Calculating difficulty…"
				progress = 1
			}
			ui.setLoadState(true, message, progress)
		})

		sort.SliceStable(maps, func(i, j int) bool {
			left := strings.ToLower(maps[i].Name)
			right := strings.ToLower(maps[j].Name)
			if left == right {
				return maps[i].Difficulty < maps[j].Difficulty
			}
			return left < right
		})

		database.UpdateStarRating(maps, func(processed, target int, message string) {
			progress := float32(0)
			if target > 0 {
				progress = float32(processed) / float32(target)
			}
			status := "Calculating star ratings…"
			if message != "" {
				status = message
			}
			ui.setLoadState(true, status, progress)
		})

		ui.l.beatmaps = maps
		ui.filteredMaps = append([]*beatmap.BeatMap(nil), maps...)
		ui.l.mapsLoaded = true
		ui.setupSongsWatcher()

		ui.setLoadState(false, fmt.Sprintf("%d difficulties ready", len(maps)), 1)
	}()
}

func (ui *gioLauncherUI) setupSongsWatcher() {
	closeWatcher()
	setupWatcher(settings.General.GetSongsDir(), func(event fsnotify.Event) {
		if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
			return
		}
		if !launcherConfig.AutoRefreshDB {
			ui.setLoadState(false, "Songs folder changed. Press Refresh to rescan the library.", 1)
			return
		}
		ui.statusMu.Lock()
		ui.status = "Songs folder changed. Refreshing the library…"
		ui.statusMu.Unlock()
		if ui.window != nil {
			ui.window.Invalidate()
		}
		ui.watchMu.Lock()
		if ui.refreshTimer != nil {
			ui.refreshTimer.Stop()
		}
		ui.refreshTimer = time.AfterFunc(1500*time.Millisecond, func() {
			ui.loadMaps()
		})
		ui.watchMu.Unlock()
	})
}

func (ui *gioLauncherUI) setLoadState(loading bool, status string, progress float32) {
	ui.statusMu.Lock()
	ui.loading = loading
	ui.status = status
	ui.loadingPct = min(max(progress, 0), 1)
	ui.statusMu.Unlock()
	if ui.window != nil {
		ui.window.Invalidate()
	}
}

func (ui *gioLauncherUI) currentStatus() (string, bool, float32, bool) {
	ui.statusMu.RLock()
	defer ui.statusMu.RUnlock()
	return ui.status, ui.loading, ui.loadingPct, ui.processActive
}

func (ui *gioLauncherUI) layout(gtx layout.Context) layout.Dimensions {
	_, loading, _, _ := ui.currentStatus()
	if !loading && ui.pendingLatestReplay && !ui.latestReplayChecked {
		ui.latestReplayChecked = true
		ui.pendingLatestReplay = false
		ui.loadLatestReplay()
	}
	paint.Fill(gtx.Ops, color.NRGBA{R: 12, G: 15, B: 23, A: 255})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(ui.drawTopBar),
		layout.Flexed(1, ui.drawWorkspace),
	)
}

func (ui *gioLauncherUI) drawTopBar(gtx layout.Context) layout.Dimensions {
	return gioSurface(gtx, color.NRGBA{R: 21, G: 25, B: 36, A: 255}, unit.Dp(0), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(76)
		gtx.Constraints.Max.Y = gtx.Dp(76)
		return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24), Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "TaktChase", 23, color.NRGBA{R: 244, G: 244, B: 255, A: 255})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "RHYTHM GAME • LAUNCHER", 10, color.NRGBA{R: 135, G: 145, B: 169, A: 255})
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					status, loading, progress, running := ui.currentStatus()
					if running {
						status = "TaktChase is running"
					}
					if loading {
						status = status + fmt.Sprintf("  ·  %d%%", int(progress*100))
					}
					return layout.Inset{Left: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ui.text(gtx, status, 12, color.NRGBA{R: 157, G: 166, B: 188, A: 255})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "v"+build.VERSION, 12, color.NRGBA{R: 142, G: 151, B: 175, A: 255})
				}),
			)
		})
	})
}

func (ui *gioLauncherUI) drawWorkspace(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(ui.drawSidebar),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(18), Right: unit.Dp(22), Bottom: unit.Dp(22), Left: unit.Dp(10)}.Layout(gtx, ui.drawPage)
		}),
	)
}

func (ui *gioLauncherUI) drawSidebar(gtx layout.Context) layout.Dimensions {
	width := gtx.Dp(224)
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	return gioSurface(gtx, color.NRGBA{R: 17, G: 21, B: 31, A: 255}, unit.Dp(0), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(22), Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "WORKSPACE", 10, color.NRGBA{R: 112, G: 123, B: 150, A: 255})
				}),
				layout.Rigid(gapHeight(12)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageHome, "Overview", "01") }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageLibrary, "Beatmap library", "02") }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageMods, "Mods & difficulty", "03") }),
				layout.Rigid(gapHeight(22)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "PREFERENCES", 10, color.NRGBA{R: 112, G: 123, B: 150, A: 255})
				}),
				layout.Rigid(gapHeight(12)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageSettings, "Settings", "04") }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageProfiles, "Profiles", "05") }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.navButton(gtx, gioPageAbout, "About", "06") }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return gioSurface(gtx, color.NRGBA{R: 29, G: 34, B: 49, A: 255}, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(13), Bottom: unit.Dp(13), Left: unit.Dp(13), Right: unit.Dp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, "ACTIVE PROFILE", 9, color.NRGBA{R: 126, G: 138, B: 166, A: 255})
								}),
								layout.Rigid(gapHeight(5)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, ui.l.bld.config, 14, color.NRGBA{R: 238, G: 240, B: 250, A: 255})
								}),
							)
						})
					})
				}),
			)
		})
	})
}

func (ui *gioLauncherUI) navButton(gtx layout.Context, page gioPage, label, number string) layout.Dimensions {
	click := ui.navButtons[page]
	selected := ui.page == page
	bg := color.NRGBA{R: 17, G: 21, B: 31, A: 0}
	fg := color.NRGBA{R: 164, G: 173, B: 195, A: 255}
	if selected {
		bg = color.NRGBA{R: 43, G: 47, B: 76, A: 255}
		fg = color.NRGBA{R: 222, G: 219, B: 255, A: 255}
	}
	dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
		return gioSurface(gtx, bg, unit.Dp(11), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(44)
			gtx.Constraints.Max.Y = gtx.Dp(44)
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return ui.text(gtx, number, 10, color.NRGBA{R: 117, G: 127, B: 153, A: 255})
					}),
					layout.Rigid(gapWidth(10)),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return ui.text(gtx, label, 13, fg)
					}),
				)
			})
		})
	})
	if click.Clicked(gtx) {
		ui.page = page
	}
	return dims
}
func (ui *gioLauncherUI) drawPage(gtx layout.Context) layout.Dimensions {
	switch ui.page {
	case gioPageHome:
		return ui.drawHome(gtx)
	case gioPageLibrary:
		return ui.drawLibrary(gtx)
	case gioPageMods:
		return ui.drawMods(gtx)
	case gioPageSettings:
		return ui.drawSettings(gtx)
	case gioPageProfiles:
		return ui.drawProfiles(gtx)
	case gioPageAbout:
		return ui.drawAbout(gtx)
	case gioPageSpeed:
		return ui.drawSpeed(gtx)
	default:
		return ui.drawHome(gtx)
	}
}

func (ui *gioLauncherUI) drawHome(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.heading(gtx, "Ready when you are", "Choose a map, tune your setup and jump into a session.")
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.modeSelector(gtx)
				}),
			)
		}),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.56, ui.drawCompactLibrary),
				layout.Rigid(gapWidth(14)),
				layout.Flexed(0.44, ui.drawSelectedMap),
			)
		}),
	)
}

func (ui *gioLauncherUI) drawLibrary(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.heading(gtx, "Beatmap library", "Search by song, artist, mapper, difficulty or set ID.")
		}),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, ui.drawCompactLibrary),
	)
}

func (ui *gioLauncherUI) modeSelector(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "PLAY"
			if launcherConfig.CurrentMode == Replay { label = "REPLAY" }
			return ui.text(gtx, "MODE  ·  "+label, 10, color.NRGBA{R: 135, G: 146, B: 174, A: 255})
		}),
		layout.Rigid(gapWidth(8)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if ui.button(gtx, &ui.modePlay, "Play", launcherConfig.CurrentMode == Play) {
				launcherConfig.CurrentMode = Play
				if ui.l.bld.currentMap != nil {
					ui.l.bld.setMap(ui.l.bld.currentMap)
				} else {
					ui.l.bld.removeReplay()
				}
				ui.l.bld.currentReplay = nil
				ui.l.bld.replayPath = ""
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Rigid(gapWidth(6)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if ui.button(gtx, &ui.modeReplay, "Replay", launcherConfig.CurrentMode == Replay) {
				launcherConfig.CurrentMode = Replay
				if ui.l.bld.currentMap != nil {
					ui.l.bld.setMap(ui.l.bld.currentMap)
				} else {
					ui.l.bld.removeReplay()
				}
				ui.l.bld.currentReplay = nil
				ui.l.bld.replayPath = ""
				ui.replayPath.SetText("")
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
	)
}

func (ui *gioLauncherUI) drawCompactLibrary(gtx layout.Context) layout.Dimensions {
	return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "YOUR MAPS", 11, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, fmt.Sprintf("%d", len(ui.filteredMaps)), 11, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
						}),
					)
				}),
				layout.Rigid(gapHeight(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					style := material.Editor(ui.theme, &ui.searchEditor, "Search maps…")
					style.Color = color.NRGBA{R: 235, G: 238, B: 247, A: 255}
					style.HintColor = color.NRGBA{R: 115, G: 126, B: 151, A: 255}
					dims := style.Layout(gtx)
					search := strings.TrimSpace(ui.searchEditor.Text())
					if search != ui.lastSearch {
						ui.lastSearch = search
						ui.filterMaps()
					}
					return dims
				}),
				layout.Rigid(gapHeight(9)),
				layout.Flexed(1, ui.drawMapRows),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if _, loading, _, _ := ui.currentStatus(); loading {
								return ui.text(gtx, "Updating library…", 10, color.NRGBA{R: 132, G: 143, B: 169, A: 255})
							}
							return ui.text(gtx, "Drop .osz files into the Songs folder to import maps.", 10, color.NRGBA{R: 132, G: 143, B: 169, A: 255})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.reloadMaps, "Refresh", false) {
								ui.loadMaps()
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				}),
			)
		})
	})
}

func (ui *gioLauncherUI) drawMapRows(gtx layout.Context) layout.Dimensions {
	status, loading, progress, _ := ui.currentStatus()
	if loading && len(ui.filteredMaps) == 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return material.Loader(ui.theme).Layout(gtx) }),
				layout.Rigid(gapHeight(12)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, status, 12, color.NRGBA{R: 166, G: 175, B: 197, A: 255}) }),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return material.ProgressBar(ui.theme, progress).Layout(gtx) }),
			)
		})
	}
	if len(ui.filteredMaps) == 0 {
		message := "No maps match this search."
		if len(ui.l.beatmaps) == 0 {
			message = "No beatmaps found. Check the Songs folder in Settings."
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, message, 13, color.NRGBA{R: 150, G: 160, B: 184, A: 255})
		})
	}

	return material.List(ui.theme, &ui.mapList).Layout(gtx, len(ui.filteredMaps), func(gtx layout.Context, index int) layout.Dimensions {
		bMap := ui.filteredMaps[index]
		key := bMap.MD5
		if key == "" {
			key = bMap.Dir + "/" + bMap.Difficulty
		}
		click := ui.mapButtons[key]
		if click == nil {
			click = new(widget.Clickable)
			ui.mapButtons[key] = click
		}
		selected := false
		if current := ui.l.bld.currentMap; current != nil {
			if current.MD5 != "" && bMap.MD5 != "" {
				selected = current.MD5 == bMap.MD5
			} else {
				selected = current.Dir == bMap.Dir && current.Difficulty == bMap.Difficulty
			}
		}
		background := color.NRGBA{R: 29, G: 35, B: 51, A: 255}
		if selected {
			background = color.NRGBA{R: 47, G: 49, B: 83, A: 255}
		}

		dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
			return gioSurface(gtx, background, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(66)
				gtx.Constraints.Max.Y = gtx.Dp(66)
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(7), Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, bMap.Name, 13, color.NRGBA{R: 237, G: 239, B: 249, A: 255})
								}),
								layout.Rigid(gapHeight(3)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, bMap.Artist+"  ·  "+bMap.Difficulty, 10, color.NRGBA{R: 145, G: 156, B: 181, A: 255})
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							stars := "—"
							if bMap.Stars >= 0 {
								stars = fmt.Sprintf("%.2f★", bMap.Stars)
							}
						return ui.text(gtx, stars, 12, color.NRGBA{R: 184, G: 175, B: 255, A: 255})
						}),
					)
				})
			})
		})
		if click.Clicked(gtx) {
			ui.l.bld.setMap(bMap)
			launcherConfig.CurrentMode = Play
			ui.page = gioPageHome
		}
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return dims
		})
	})
}
func (ui *gioLauncherUI) drawSelectedMap(gtx layout.Context) layout.Dimensions {
	if launcherConfig.CurrentMode == Replay {
		return ui.drawReplayPanel(gtx)
	}
	return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(18), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "SELECTED BEATMAP", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
				}),
				layout.Rigid(gapHeight(14)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.drawMapDetails(gtx)
				}),
				layout.Rigid(gapHeight(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "SESSION", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
				}),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.openSpeed, "Speed & timing", false) {
								ui.speedEditor.SetText(strconv.FormatFloat(ui.l.bld.speed.value, 'f', 2, 64))
								ui.pitchEditor.SetText(strconv.FormatFloat(ui.l.bld.pitch.value, 'f', 2, 64))
								ui.offsetEditor.SetText(strconv.FormatInt(int64(ui.l.bld.offset.value), 10))
								ui.startEditor.SetText(strconv.FormatInt(int64(ui.l.bld.start.value), 10))
								ui.endEditor.SetText(strconv.FormatInt(int64(ui.l.bld.end.value), 10))
								ui.page = gioPageSpeed
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(gapWidth(8)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.openMods, "Mods", false) {
								ui.page = gioPageMods
						}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				}),
				layout.Rigid(gapHeight(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.l.canStart() {
						return ui.text(gtx, "Ready to launch", 11, color.NRGBA{R: 140, G: 151, B: 176, A: 255})
					}
					return ui.text(gtx, "Choose a beatmap to continue", 11, color.NRGBA{R: 140, G: 151, B: 176, A: 255})
				}),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					_, _, _, running := ui.currentStatus()
					if running {
						if ui.button(gtx, &ui.cancelButton, "Stop TaktChase", false) {
							ui.stopGame()
						}
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}
					return ui.primaryButton(gtx, &ui.startButton, "Start TaktChase", ui.l.canStart())
				}),
			)
		})
	})
}
func (ui *gioLauncherUI) drawSpeed(gtx layout.Context) layout.Dimensions {
	field := func(gtx layout.Context, title, hint string, editor *widget.Editor) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.text(gtx, title, 11, color.NRGBA{R: 190, G: 198, B: 216, A: 255})
			}),
			layout.Rigid(gapHeight(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				style := material.Editor(ui.theme, editor, hint)
				style.TextSize = unit.Sp(13)
				return style.Layout(gtx)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.heading(gtx, "Speed, pitch & timing", "Tune the session range, audio speed and local offset.")
		}),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(18), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "PLAYBACK", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
						}),
						layout.Rigid(gapHeight(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return field(gtx, "Speed multiplier", "1.00", &ui.speedEditor) }),
								layout.Rigid(gapWidth(12)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return field(gtx, "Pitch multiplier", "1.00", &ui.pitchEditor) }),
							)
						}),
						layout.Rigid(gapHeight(18)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "TIME RANGE & OFFSET", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255}) }),
						layout.Rigid(gapHeight(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return field(gtx, "Start (seconds)", "0", &ui.startEditor) }),
								layout.Rigid(gapWidth(12)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return field(gtx, "End (seconds)", "0", &ui.endEditor) }),
								layout.Rigid(gapWidth(12)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return field(gtx, "Audio offset (ms)", "0", &ui.offsetEditor) }),
							)
						}),
						layout.Rigid(gapHeight(12)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							state := ui.settingsBool["skip-map-intro"]
							if state == nil { state = new(widget.Bool); ui.settingsBool["skip-map-intro"] = state }
							state.Value = ui.l.bld.skip
							style := material.CheckBox(ui.theme, state, "Skip intro and jump to the drain time")
							style.Layout(gtx)
							ui.l.bld.skip = state.Value
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, "Speed and pitch accept values from 0.10 to 3.00.", 10, color.NRGBA{R: 130, G: 142, B: 167, A: 255})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if ui.button(gtx, &ui.cancelButton, "Reset", false) {
										ui.l.bld.speed.value, ui.l.bld.speed.changed = ui.l.bld.speed.ogValue, false
										ui.l.bld.pitch.value, ui.l.bld.pitch.changed = ui.l.bld.pitch.ogValue, false
										ui.l.bld.offset.value, ui.l.bld.offset.changed = ui.l.bld.offset.ogValue, false
										ui.l.bld.start.value, ui.l.bld.start.changed = ui.l.bld.start.ogValue, false
										ui.l.bld.end.value, ui.l.bld.end.changed = ui.l.bld.end.ogValue, false
										ui.l.bld.skip = false
										ui.speedEditor.SetText(strconv.FormatFloat(ui.l.bld.speed.value, 'f', 2, 64))
										ui.pitchEditor.SetText(strconv.FormatFloat(ui.l.bld.pitch.value, 'f', 2, 64))
										ui.offsetEditor.SetText(strconv.FormatInt(int64(ui.l.bld.offset.value), 10))
										ui.startEditor.SetText(strconv.FormatInt(int64(ui.l.bld.start.value), 10))
										ui.endEditor.SetText(strconv.FormatInt(int64(ui.l.bld.end.value), 10))
									}
									return layout.Dimensions{Size: gtx.Constraints.Min}
								}),
								layout.Rigid(gapWidth(8)),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if ui.button(gtx, &ui.saveSettings, "Apply", true) {
										ui.applySpeedAndTiming()
										ui.page = gioPageHome
									}
									return layout.Dimensions{Size: gtx.Constraints.Min}
								}),
							)
						}),
					)
				})
			})
		}),
	)
}

func (ui *gioLauncherUI) applySpeedAndTiming() {
	b := ui.l.bld
	if value, err := strconv.ParseFloat(strings.TrimSpace(ui.speedEditor.Text()), 64); err == nil {
		b.speed.value = min(max(value, 0.1), 3.0)
		b.speed.changed = math.Abs(b.speed.value-b.speed.ogValue) > 0.001
	}
	if value, err := strconv.ParseFloat(strings.TrimSpace(ui.pitchEditor.Text()), 64); err == nil {
		b.pitch.value = min(max(value, 0.1), 3.0)
		b.pitch.changed = math.Abs(b.pitch.value-b.pitch.ogValue) > 0.001
	}
	if value, err := strconv.ParseInt(strings.TrimSpace(ui.startEditor.Text()), 10, 32); err == nil {
		if b.currentMap != nil {
			value = min(max(value, 0), int64(b.currentMap.Length/1000))
		}
		b.start.value = int32(value)
		b.start.changed = b.start.value != b.start.ogValue
	}
	if value, err := strconv.ParseInt(strings.TrimSpace(ui.endEditor.Text()), 10, 32); err == nil {
		if b.currentMap != nil {
			value = min(max(value, 1), int64(b.currentMap.Length/1000))
		}
		b.end.value = int32(value)
		b.end.changed = b.end.value != b.end.ogValue
	}
	if value, err := strconv.ParseInt(strings.TrimSpace(ui.offsetEditor.Text()), 10, 32); err == nil {
		b.offset.value = int32(value)
		b.offset.changed = b.offset.value != b.offset.ogValue
	}
	if b.end.value > 0 && b.start.value >= b.end.value {
		b.start.value = max(int32(0), b.end.value-1)
		b.start.changed = b.start.value != b.start.ogValue
		ui.startEditor.SetText(strconv.FormatInt(int64(b.start.value), 10))
	}
	if b.currentMap != nil && b.offset.value != int32(b.currentMap.LocalOffset) {
		b.currentMap.LocalOffset = int(b.offset.value)
		database.UpdateLocalOffset(b.currentMap)
	}
	ui.setLoadState(false, "Session settings applied.", 1)
}

func (ui *gioLauncherUI) drawReplayPanel(gtx layout.Context) layout.Dimensions {
	return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(18), Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "REPLAY SESSION", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
				}),
				layout.Rigid(gapHeight(12)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					style := material.Editor(ui.theme, &ui.replayPath, "Path to .osr replay")
					style.TextSize = unit.Sp(12)
					return style.Layout(gtx)
				}),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.openReplay, "Browse…", false) {
								ui.chooseReplay()
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(gapWidth(8)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.loadReplay, "Load replay", true) {
								ui.loadReplayFile(strings.TrimSpace(ui.replayPath.Text()))
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				}),
				layout.Rigid(gapHeight(18)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if ui.l.bld.currentReplay == nil || ui.l.bld.currentMap == nil {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "Load a replay to see its player and beatmap details.", 12, color.NRGBA{R: 145, G: 156, B: 181, A: 255})
						})
					}
					replay := ui.l.bld.currentReplay
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, ui.l.bld.currentMap.Name, 20, color.NRGBA{R: 242, G: 244, B: 255, A: 255})
						}),
						layout.Rigid(gapHeight(5)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, ui.l.bld.currentMap.Artist+" · "+ui.l.bld.currentMap.Difficulty, 12, color.NRGBA{R: 151, G: 162, B: 187, A: 255})
						}),
						layout.Rigid(gapHeight(12)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, "Played by "+replay.Username+"  ·  "+fmt.Sprintf("%d score", replay.Score), 12, color.NRGBA{R: 184, G: 175, B: 255, A: 255})
						}),
					)
				}),
				layout.Rigid(gapHeight(10)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.cancelButton, "Back", false) {
								ui.page = gioPageHome
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(gapWidth(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							_, _, _, running := ui.currentStatus()
							if running {
								if ui.button(gtx, &ui.startButton, "Stop", false) {
									ui.stopGame()
								}
							} else {
								ui.primaryButton(gtx, &ui.startButton, "Watch replay", ui.l.canStart())
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				}),
			)
		})
	})
}

func (ui *gioLauncherUI) drawMapDetails(gtx layout.Context) layout.Dimensions {
	b := ui.l.bld.currentMap
	if b == nil {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "No beatmap selected", 20, color.NRGBA{R: 230, G: 233, B: 245, A: 255})
				}),
				layout.Rigid(gapHeight(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "Pick a difficulty from your library.", 12, color.NRGBA{R: 145, G: 156, B: 181, A: 255})
				}),
			)
		})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, b.Name, 22, color.NRGBA{R: 245, G: 245, B: 255, A: 255})
		}),
		layout.Rigid(gapHeight(5)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, b.Artist+"  ·  "+b.Creator, 12, color.NRGBA{R: 151, G: 162, B: 187, A: 255})
		}),
		layout.Rigid(gapHeight(14)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			stars := "N/A"
			if b.Stars >= 0 { stars = fmt.Sprintf("%.2f ★", b.Stars) }
			return gioSurface(gtx, color.NRGBA{R: 43, G: 43, B: 70, A: 255}, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(11), Bottom: unit.Dp(11), Left: unit.Dp(13), Right: unit.Dp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, stars, 17, color.NRGBA{R: 205, G: 197, B: 255, A: 255}) }),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, fmtDuration(b.Length), 12, color.NRGBA{R: 180, G: 188, B: 207, A: 255}) }),
					)
				})
			})
		}),
		layout.Rigid(gapHeight(14)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.statGrid(gtx, b) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, "ACTIVE MODS   "+ui.l.bld.diff.GetModString(), 11, color.NRGBA{R: 145, G: 156, B: 183, A: 255})
		}),
	)
}

func (ui *gioLauncherUI) statGrid(gtx layout.Context, b *beatmap.BeatMap) layout.Dimensions {
	stat := func(label, value string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return gioSurface(gtx, color.NRGBA{R: 31, G: 37, B: 53, A: 255}, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, label, 9, color.NRGBA{R: 127, G: 139, B: 164, A: 255}) }),
						layout.Rigid(gapHeight(4)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, value, 14, color.NRGBA{R: 230, G: 234, B: 247, A: 255}) }),
					)
				})
			})
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, stat("BPM", fmtBPM(b.MinBPM, b.MaxBPM))),
				layout.Rigid(gapWidth(8)),
				layout.Flexed(1, stat("OBJECTS", strconv.Itoa(b.Circles+b.Sliders+b.Spinners))),
			)
		}),
		layout.Rigid(gapHeight(8)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, stat("AR", fmt.Sprintf("%.1f", b.Diff.GetAR()))),
				layout.Rigid(gapWidth(8)),
				layout.Flexed(1, stat("OD", fmt.Sprintf("%.1f", b.Diff.GetOD()))),
			)
		}),
		layout.Rigid(gapHeight(8)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, stat("CS", fmt.Sprintf("%.1f", b.Diff.GetCS()))),
				layout.Rigid(gapWidth(8)),
				layout.Flexed(1, stat("HP", fmt.Sprintf("%.1f", b.Diff.GetHP()))),
			)
		}),
	)
}

func (ui *gioLauncherUI) drawMods(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.heading(gtx, "Mods & difficulty", "Toggle gameplay modifiers. Incompatible combinations are cleared automatically.")
		}),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(ui.theme, &ui.modsList).Layout(gtx, len(gioMods), func(gtx layout.Context, index int) layout.Dimensions {
				mod := gioMods[index]
				active := ui.l.bld.diff.Mods.Active(mod.mod)
				bg := color.NRGBA{R: 24, G: 29, B: 42, A: 255}
				if active {
					bg = color.NRGBA{R: 48, G: 47, B: 81, A: 255}
				}
				click := ui.modButtons[mod.mod]
				dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
					return gioSurface(gtx, bg, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(54)
						gtx.Constraints.Max.Y = gtx.Dp(54)
						return layout.Inset{Left: unit.Dp(13), Right: unit.Dp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.text(gtx, mod.acronym, 12, color.NRGBA{R: 190, G: 181, B: 255, A: 255})
								}),
								layout.Rigid(gapWidth(12)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return ui.text(gtx, mod.label, 13, color.NRGBA{R: 229, G: 233, B: 246, A: 255})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return ui.text(gtx, mod.group, 9, color.NRGBA{R: 130, G: 141, B: 166, A: 255})
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									state := "OFF"
									if active {
										state = "ON"
								}
									return ui.text(gtx, state, 10, color.NRGBA{R: 174, G: 167, B: 255, A: 255})
								}),
							)
						})
					})
				})
				if click.Clicked(gtx) {
					ui.toggleMod(mod.mod)
				}
				return layout.Inset{Bottom: unit.Dp(7), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return dims
				})
			})
		}),
		layout.Rigid(gapHeight(10)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "Current: "+ui.l.bld.diff.GetModString(), 12, color.NRGBA{R: 165, G: 174, B: 197, A: 255})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.button(gtx, &ui.cancelButton, "Back", false) {
						ui.page = gioPageHome
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Rigid(gapWidth(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.button(gtx, &ui.startButton, "Start", ui.l.canStart()) {
						ui.launchGame()
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
			)
		}),
	)
}
func (ui *gioLauncherUI) toggleMod(mod difficulty.Modifier) {
	diff := ui.l.bld.diff
	if diff.Mods.Active(mod) {
		diff.RemoveMod(mod)
		return
	}
	conflicts := diff.Mods & mod.GetStableIncompatibleMods()
	if conflicts != difficulty.None {
		diff.RemoveMod(conflicts)
	}
	switch mod {
	case difficulty.Nightcore:
		diff.AddMod(difficulty.DoubleTime | difficulty.Nightcore)
	case difficulty.Daycore:
		diff.AddMod(difficulty.HalfTime | difficulty.Daycore)
	case difficulty.Perfect:
		diff.AddMod(difficulty.SuddenDeath | difficulty.Perfect)
	default:
		diff.AddMod(mod)
	}
}

func (ui *gioLauncherUI) drawSettings(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.heading(gtx, "Settings", "Configure graphics, sound, input, gameplay and launcher preferences.")
		}),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					width := gtx.Dp(190)
					gtx.Constraints.Min.X = width
					gtx.Constraints.Max.X = width
					return gioSurface(gtx, color.NRGBA{R: 22, G: 27, B: 39, A: 255}, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.List(ui.theme, &ui.settingsNavList).Layout(gtx, len(gioSettingsCategories), func(gtx layout.Context, index int) layout.Dimensions {
								category := gioSettingsCategories[index]
								click := ui.settingsButtons[category]
								selected := ui.settingsCategory == category
								bg := color.NRGBA{R: 22, G: 27, B: 39, A: 0}
								if selected { bg = color.NRGBA{R: 49, G: 48, B: 82, A: 255} }
								dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
									return gioSurface(gtx, bg, unit.Dp(9), func(gtx layout.Context) layout.Dimensions {
										gtx.Constraints.Min.Y = gtx.Dp(40)
										gtx.Constraints.Max.Y = gtx.Dp(40)
										return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return ui.text(gtx, category, 11, color.NRGBA{R: 205, G: 211, B: 229, A: 255})
										})
									})
								})
								if click.Clicked(gtx) && ui.settingsCategory != category {
									ui.settingsCategory = category
									ui.rebuildSettingsFields()
								}
								return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return dims })
							})
						})
					})
				}),
				layout.Rigid(gapWidth(14)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if ui.settingsCategory == "Launcher" {
						return ui.drawLauncherPreferences(gtx)
					}
					return ui.drawConfigFields(gtx)
				}),
			)
		}),
		layout.Rigid(gapHeight(12)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.button(gtx, &ui.saveSettings, "Save settings", true) {
						ui.saveAllSettings()
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
			)
		}),
	)
}
func (ui *gioLauncherUI) drawConfigFields(gtx layout.Context) layout.Dimensions {
	return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(14), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, ui.settingsCategory, 16, color.NRGBA{R: 237, G: 239, B: 249, A: 255}) }),
				layout.Rigid(gapHeight(10)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(ui.settingsFields) == 0 {
						return ui.text(gtx, "No editable fields in this section.", 12, color.NRGBA{R: 150, G: 160, B: 183, A: 255})
					}
					return material.List(ui.theme, &ui.settingsFieldList).Layout(gtx, len(ui.settingsFields), func(gtx layout.Context, index int) layout.Dimensions {
						return ui.drawSettingField(gtx, ui.settingsFields[index])
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if ui.settingsCategory != "Credentials" { return layout.Dimensions{} }
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.authorize, "Authorize with osu!", true) {
								settings.SaveCredentials(false)
								ui.setLoadState(true, "Waiting for osu! authorization…", 0)
								go osuapi.Authorize(func(result osuapi.AuthResult, message string) {
									settings.SaveCredentials(false)
									if result == osuapi.AuthError { ui.setLoadState(false, "Authorization failed: "+message, 0) } else { ui.setLoadState(false, message, 1) }
								})
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(gapWidth(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.resetTokens, "Reset tokens", false) {
								settings.Credentails.AccessToken = ""
								settings.Credentails.RefreshToken = ""
								settings.Credentails.Expiry = time.Unix(0, 0)
								settings.SaveCredentials(true)
								ui.setLoadState(false, "OAuth tokens reset.", 1)
							}
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Rigid(gapWidth(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ui.button(gtx, &ui.credentialHelp, "Help", false) { platform.OpenURL("https://github.com/shntsk/TaktChase/wiki/APIv2-Tutorial") }
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				}),
			)
		})
	})
}

func (ui *gioLauncherUI) drawSettingField(gtx layout.Context, field *gioSettingField) layout.Dimensions {
	if field.group {
		return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, field.label, 12, color.NRGBA{R: 194, G: 187, B: 255, A: 255})
		})
	}
	return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return gioSurface(gtx, color.NRGBA{R: 29, G: 35, B: 50, A: 255}, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(54)
			gtx.Constraints.Max.Y = gtx.Dp(54)
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(7), Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				switch field.value.Kind() {
				case reflect.Bool:
					field.bool.Value = field.value.Bool()
					style := material.Switch(ui.theme, &field.bool, field.label)
					style.Layout(gtx)
					field.value.SetBool(field.bool.Value)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				case reflect.String:
					if len(field.options) > 0 {
						current := field.value.String()
						label := current
						for _, option := range field.options {
							if option.value == current { label = option.label; break }
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, field.label, 11, color.NRGBA{R: 190, G: 198, B: 216, A: 255}) }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if ui.button(gtx, &field.click, label+"  ▾", false) {
									index := 0
								for i, option := range field.options { if option.value == current { index = i; break } }
									field.value.SetString(field.options[(index+1)%len(field.options)].value)
								}
								return layout.Dimensions{Size: gtx.Constraints.Min}
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(0.36, func(gtx layout.Context) layout.Dimensions {
							return ui.text(gtx, field.label, 11, color.NRGBA{R: 190, G: 198, B: 216, A: 255})
						}),
						layout.Rigid(gapWidth(10)),
						layout.Flexed(0.64, func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(ui.theme, &field.editor, "Value")
							style.TextSize = unit.Sp(12)
							style.Layout(gtx)
							if field.value.CanSet() { field.value.SetString(field.editor.Text()) }
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
					reflect.Float32, reflect.Float64:
					if len(field.options) > 0 {
						current := strconv.FormatInt(field.value.Int(), 10)
						label := current
						for _, option := range field.options {
							if option.value == current { label = option.label; break }
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, field.label, 11, color.NRGBA{R: 190, G: 198, B: 216, A: 255}) }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								clicked := ui.button(gtx, &field.click, label+"  ▾", false)
								if clicked {
									index := 0
									for i, option := range field.options { if option.value == current { index = i; break } }
									next := field.options[(index+1)%len(field.options)]
									if number, err := strconv.ParseInt(next.value, 10, 64); err == nil && field.value.CanSet() { field.value.SetInt(number) }
								}
								return layout.Dimensions{Size: gtx.Constraints.Min}
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(0.36, func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, field.label, 11, color.NRGBA{R: 190, G: 198, B: 216, A: 255}) }),
						layout.Rigid(gapWidth(10)),
						layout.Flexed(0.64, func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(ui.theme, &field.editor, "Number")
							style.TextSize = unit.Sp(12)
							style.Layout(gtx)
							ui.applyNumericEditor(field)
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
					)
				default:
					return ui.text(gtx, field.label, 11, color.NRGBA{R: 177, G: 186, B: 206, A: 255})
				}
			})
		})
	})
}

func (ui *gioLauncherUI) drawLauncherPreferences(gtx layout.Context) layout.Dimensions {
	return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(17), Right: unit.Dp(17)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			options := []struct {
				key   string
				label string
				value *bool
			}{
				{"CheckForUpdates", "Check for updates on startup", &launcherConfig.CheckForUpdates},
				{"LoadLatestReplay", "Load the latest replay on startup", &launcherConfig.LoadLatestReplay},
				{"SkipMapUpdate", "Skip full map comparison for faster startup", &launcherConfig.SkipMapUpdate},
				{"AutoRefreshDB", "Refresh the library when the Songs folder changes", &launcherConfig.AutoRefreshDB},
				{"ShowJSONPaths", "Show JSON paths in the settings editor", &launcherConfig.ShowJSONPaths},
				{"ShowFileAfter", "Show exported files after rendering", &launcherConfig.ShowFileAfter},
				{"PreviewSelected", "Preview selected maps", &launcherConfig.PreviewSelected},
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "Launcher preferences", 16, color.NRGBA{R: 237, G: 239, B: 249, A: 255}) }),
				layout.Rigid(gapHeight(12)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.text(gtx, "These options are stored separately from the gameplay profile.", 11, color.NRGBA{R: 145, G: 156, B: 181, A: 255})
				}),
				layout.Rigid(gapHeight(10)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, launcherBoolWidgets(ui, options)...)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "Preview volume", 12, color.NRGBA{R: 190, G: 198, B: 216, A: 255}) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(ui.theme, &ui.launcherVolume, "0–100")
							style.TextSize = unit.Sp(12)
							dims := style.Layout(gtx)
							if volume, err := strconv.Atoi(ui.launcherVolume.Text()); err == nil {
								launcherConfig.PreviewVolume = math.Max(0, math.Min(1, float64(volume)/100))
							}
							return dims
						}),
					)
				}),
			)
		})
	})
}

func launcherBoolWidgets(ui *gioLauncherUI, options []struct{ key, label string; value *bool }) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(options)*2)
	for _, option := range options {
		current := option
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			state := ui.settingsBool[current.key]
			if state == nil {
				state = new(widget.Bool)
				ui.settingsBool[current.key] = state
			}
			state.Value = *current.value
			style := material.CheckBox(ui.theme, state, current.label)
			style.Layout(gtx)
			*current.value = state.Value
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}))
		children = append(children, layout.Rigid(gapHeight(4)))
	}
	return children
}

func (ui *gioLauncherUI) rebuildSettingsFields() {
	ui.settingsFields = nil
	if ui.l.currentConfig == nil || ui.settingsCategory == "Launcher" { return }
	combined := ui.l.currentConfig.GetCombined()
	root := reflect.ValueOf(combined)
	if root.Kind() == reflect.Ptr { root = root.Elem() }
	for i := 0; i < root.NumField(); i++ {
		sf := root.Type().Field(i)
		if sf.Name == ui.settingsCategory {
			ui.collectSettingsFields(sf.Name, root.Field(i))
			break
		}
	}
}

func (ui *gioLauncherUI) collectSettingsFields(path string, value reflect.Value) {
	if value.Kind() == reflect.Ptr {
		if value.IsNil() { return }
		value = value.Elem()
	}
	if !value.IsValid() { return }
	if value.Kind() == reflect.Struct {
		for i := 0; i < value.NumField(); i++ {
			sf := value.Type().Field(i)
			if sf.PkgPath != "" || sf.Tag.Get("skip") == "true" { continue }
			name := sf.Name
			if jsonName := strings.Split(sf.Tag.Get("json"), ",")[0]; jsonName == "-" { continue }
			label := sf.Tag.Get("label")
			if label == "" { label = gioHumanize(name) }
			child := value.Field(i)
			childPath := path + "." + name
			switch child.Kind() {
			case reflect.Struct, reflect.Ptr:
				if child.Kind() == reflect.Ptr && child.IsNil() { continue }
				ui.settingsFields = append(ui.settingsFields, &gioSettingField{key: childPath + ".group", label: label, group: true})
				ui.collectSettingsFields(childPath, child)
			case reflect.Bool:
				if child.CanSet() {
					f := &gioSettingField{key: childPath, label: label, value: child}
					f.bool.Value = child.Bool()
					ui.settingsFields = append(ui.settingsFields, f)
				}
			case reflect.String:
				if child.CanSet() {
					f := &gioSettingField{key: childPath, label: label, value: child}
					f.editor.SingleLine = true
					if sf.Tag.Get("password") == "true" {
						f.editor.Mask = '•'
					}
					f.editor.SetText(child.String())
					f.options = parseGioOptions(sf.Tag.Get("combo"))
					ui.settingsFields = append(ui.settingsFields, f)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if child.CanSet() {
					f := &gioSettingField{key: childPath, label: label, value: child}
					f.editor.SingleLine = true
					f.editor.SetText(strconv.FormatInt(child.Int(), 10))
					f.options = parseGioOptions(sf.Tag.Get("combo"))
					ui.settingsFields = append(ui.settingsFields, f)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if child.CanSet() {
					f := &gioSettingField{key: childPath, label: label, value: child}
					f.editor.SingleLine = true
					f.editor.SetText(strconv.FormatUint(child.Uint(), 10))
					ui.settingsFields = append(ui.settingsFields, f)
				}
			case reflect.Float32, reflect.Float64:
				if child.CanSet() {
					f := &gioSettingField{key: childPath, label: label, value: child}
					f.editor.SingleLine = true
					f.editor.SetText(strconv.FormatFloat(child.Float(), 'f', 3, 64))
					ui.settingsFields = append(ui.settingsFields, f)
				}
			}
		}
	}
}

func (ui *gioLauncherUI) applyNumericEditor(field *gioSettingField) {
	value := strings.TrimSpace(field.editor.Text())
	switch field.value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil { field.value.SetInt(parsed) }
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil { field.value.SetUint(parsed) }
	case reflect.Float32, reflect.Float64:
		if parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64); err == nil { field.value.SetFloat(parsed) }
	}
}

func parseGioOptions(raw string) []gioSettingOption {
	if raw == "" { return nil }
	var options []gioSettingOption
	for _, part := range strings.Split(raw, ",") {
		bits := strings.SplitN(strings.TrimSpace(part), "|", 2)
		if len(bits) != 2 || bits[1] == "custom" { continue }
		options = append(options, gioSettingOption{value: bits[0], label: bits[1]})
	}
	return options
}

func gioHumanize(value string) string {
	var out strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' { out.WriteByte(' ') }
		out.WriteRune(r)
	}
	return out.String()
}

func (ui *gioLauncherUI) drawProfiles(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.heading(gtx, "Profiles", "Keep separate setups for different play styles and installations.") }),
		layout.Rigid(gapHeight(16)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(15), Bottom: unit.Dp(15), Left: unit.Dp(15), Right: unit.Dp(15)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "AVAILABLE PROFILES", 10, color.NRGBA{R: 145, G: 156, B: 183, A: 255}) }),
						layout.Rigid(gapHeight(10)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.List(ui.theme, &ui.profileList).Layout(gtx, len(ui.l.configList), func(gtx layout.Context, index int) layout.Dimensions {
								name := ui.l.configList[index]
								click := ui.profileButtons[name]
								if click == nil { click = new(widget.Clickable); ui.profileButtons[name] = click }
								active := name == ui.l.bld.config
								bg := color.NRGBA{R: 30, G: 36, B: 52, A: 255}
								if active { bg = color.NRGBA{R: 48, G: 47, B: 82, A: 255} }
								dims := material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
									return gioSurface(gtx, bg, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
										gtx.Constraints.Min.Y = gtx.Dp(48)
										gtx.Constraints.Max.Y = gtx.Dp(48)
										return layout.Inset{Left: unit.Dp(13), Right: unit.Dp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
												layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, name, 13, color.NRGBA{R: 232, G: 235, B: 247, A: 255}) }),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													if active { return ui.text(gtx, "ACTIVE", 9, color.NRGBA{R: 189, G: 179, B: 255, A: 255}) }
													return layout.Dimensions{}
												}),
											)
										})
									})
								})
								if click.Clicked(gtx) { ui.selectProfile(name) }
								return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return dims })
							})
						}),
						layout.Rigid(gapHeight(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(ui.theme, &ui.profileName, "New profile name")
							return style.Layout(gtx)
						}),
						layout.Rigid(gapHeight(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									if ui.button(gtx, &ui.createProfile, "Create", true) { ui.createProfileFromCurrent() }
										return layout.Dimensions{Size: gtx.Constraints.Min}
								}),
								layout.Rigid(gapWidth(8)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									if ui.button(gtx, &ui.cloneProfile, "Clone current", false) { ui.cloneCurrentProfile() }
										return layout.Dimensions{Size: gtx.Constraints.Min}
								}),
								layout.Rigid(gapWidth(8)),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									if ui.button(gtx, &ui.deleteProfile, "Delete", false) { ui.deleteCurrentProfile() }
										return layout.Dimensions{Size: gtx.Constraints.Min}
								}),
							)
						}),
					)
				})
			})
		}),
	)
}
func (ui *gioLauncherUI) selectProfile(name string) {
	oldSongsDir := settings.General.GetSongsDir()
	conf, err := ui.l.loadConfig(name)
	if err != nil {
		ui.setLoadState(false, "Could not load profile: "+err.Error(), 0)
		return
	}
	ui.l.currentConfig = conf
	ui.l.bld.config = name
	*launcherConfig.Profile = name
	settings.General.OsuSongsDir = conf.General.OsuSongsDir
	settings.General.OsuSkinsDir = conf.General.OsuSkinsDir
	settings.General.OsuReplaysDir = conf.General.OsuReplaysDir
	saveLauncherConfig()
	ui.rebuildSettingsFields()
	if oldSongsDir != settings.General.GetSongsDir() {
		ui.loadMaps()
	} else {
		ui.setLoadState(false, "Profile switched to "+name, 1)
	}
}

func (ui *gioLauncherUI) createProfileFromCurrent() {
	name := safeGioProfileName(ui.profileName.Text())
	if name == "" { ui.setLoadState(false, "Enter a valid profile name.", 0); return }
	path := filepath.Join(env.ConfigDir(), name+".json")
	if _, err := os.Stat(path); err == nil { ui.setLoadState(false, "That profile already exists.", 0); return }
	conf, err := ui.l.loadConfig(ui.l.bld.config)
	if err != nil { ui.setLoadState(false, err.Error(), 0); return }
	conf.Save(path, true)
	ui.l.createConfigList()
	ui.selectProfile(name)
	ui.profileName.SetText("")
}

func (ui *gioLauncherUI) cloneCurrentProfile() {
	ui.createProfileFromCurrent()
}

func (ui *gioLauncherUI) deleteCurrentProfile() {
	name := ui.l.bld.config
	if name == "default" || name == "" { ui.setLoadState(false, "The default profile cannot be deleted.", 0); return }
	if err := os.Remove(filepath.Join(env.ConfigDir(), name+".json")); err != nil { ui.setLoadState(false, err.Error(), 0); return }
	ui.l.createConfigList()
	ui.selectProfile("default")
}

func safeGioProfileName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.TrimSuffix(value, ".json")
	return value
}

func (ui *gioLauncherUI) drawAbout(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return gioSurface(gtx, color.NRGBA{R: 23, G: 28, B: 41, A: 255}, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(36), Bottom: unit.Dp(36), Left: unit.Dp(40), Right: unit.Dp(40)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "TaktChase", 30, color.NRGBA{R: 243, G: 244, B: 255, A: 255}) }),
					layout.Rigid(gapHeight(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "A rhythm-game project focused on timing, precision and control.", 12, color.NRGBA{R: 151, G: 162, B: 187, A: 255}) }),
					layout.Rigid(gapHeight(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ui.text(gtx, "Version "+build.VERSION, 11, color.NRGBA{R: 128, G: 139, B: 165, A: 255}) }),
					layout.Rigid(gapHeight(22)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						clicked := ui.button(gtx, &ui.aboutGitHub, "Project on GitHub", true)
						if clicked { platform.OpenURL("https://github.com/shntsk/TaktChase") }
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
				)
			})
		})
	})
}

func (ui *gioLauncherUI) heading(gtx layout.Context, title, subtitle string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, title, 23, color.NRGBA{R: 241, G: 243, B: 253, A: 255})
		}),
		layout.Rigid(gapHeight(5)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.text(gtx, subtitle, 11, color.NRGBA{R: 140, G: 151, B: 176, A: 255})
		}),
	)
}

func (ui *gioLauncherUI) button(gtx layout.Context, click *widget.Clickable, label string, primary bool) bool {
	style := material.Button(ui.theme, click, label)
	style.CornerRadius = unit.Dp(12)
	style.TextSize = unit.Sp(12)
	style.Inset = layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(13), Right: unit.Dp(13)}
	if primary {
		style.Background = color.NRGBA{R: 126, G: 111, B: 255, A: 255}
		style.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	} else {
		style.Background = color.NRGBA{R: 43, G: 49, B: 67, A: 255}
		style.Color = color.NRGBA{R: 220, G: 225, B: 240, A: 255}
	}
	style.Layout(gtx)
	return click.Clicked(gtx)
}

func (ui *gioLauncherUI) primaryButton(gtx layout.Context, click *widget.Clickable, label string, enabled bool) layout.Dimensions {
	style := material.Button(ui.theme, click, label)
	style.CornerRadius = unit.Dp(14)
	style.TextSize = unit.Sp(14)
	style.Inset = layout.Inset{Top: unit.Dp(15), Bottom: unit.Dp(15), Left: unit.Dp(18), Right: unit.Dp(18)}
	if enabled {
		style.Background = color.NRGBA{R: 126, G: 111, B: 255, A: 255}
		style.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	} else {
		style.Background = color.NRGBA{R: 50, G: 55, B: 72, A: 255}
		style.Color = color.NRGBA{R: 135, G: 144, B: 164, A: 255}
	}
	dims := style.Layout(gtx)
	if click.Clicked(gtx) && enabled { ui.launchGame() }
	return dims
}

func (ui *gioLauncherUI) text(gtx layout.Context, value string, size int, foreground color.NRGBA) layout.Dimensions {
	style := material.Label(ui.theme, unit.Sp(size), value)
	style.Color = foreground
	return style.Layout(gtx)
}

func gioSurface(gtx layout.Context, background color.NRGBA, radius unit.Dp, child layout.Widget) layout.Dimensions {
	recorder := op.Record(gtx.Ops)
	dims := child(gtx)
	call := recorder.Stop()

	rounded := clip.RRect{
		Rect: image.Rectangle{Max: dims.Size},
		NE: radius, NW: radius, SE: radius, SW: radius,
	}
	paint.FillShape(gtx.Ops, background, rounded.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return dims
}
func gapHeight(value int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(0, gtx.Dp(value))} }
}

func gapWidth(value int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(gtx.Dp(value), 0)} }
}

func (ui *gioLauncherUI) filterMaps() {
	query := strings.ToLower(strings.TrimSpace(ui.searchEditor.Text()))
	filtered := make([]*beatmap.BeatMap, 0, len(ui.l.beatmaps))
	for _, b := range ui.l.beatmaps {
		if query != "" {
			haystack := strings.ToLower(b.Artist+" "+b.Name+" "+b.Difficulty+" "+b.Creator+" "+strconv.FormatInt(b.SetID, 10))
			if !strings.Contains(haystack, query) { continue }
		}
		filtered = append(filtered, b)
	}
	sortMaps(filtered, launcherConfig.SortMapsBy)
	ui.filteredMaps = filtered
	if ui.window != nil { ui.window.Invalidate() }
}

func (ui *gioLauncherUI) chooseReplay() {
	dir := env.DataDir()
	if ui.l.currentConfig != nil { dir = ui.l.currentConfig.General.GetReplaysDir() }
	path, err := dialog.File().Filter("osu! replay file (*.osr)", "osr").Title("Select replay file").SetStartDir(dir).Load()
	if err != nil { return }
	ui.replayPath.SetText(path)
	ui.loadReplayFile(path)
}

func (ui *gioLauncherUI) loadReplayFile(path string) {
	replay, err := ui.l.loadReplay(path)
	if err != nil { ui.setLoadState(false, "Could not load replay: "+err.Error(), 0); return }
	for _, b := range ui.l.beatmaps {
		if strings.EqualFold(b.MD5, replay.parsedReplay.BeatmapMD5) {
			launcherConfig.CurrentMode = Replay
			ui.l.bld.replayPath = path
			ui.l.bld.setMap(b)
			ui.l.bld.setReplay(replay.parsedReplay)
			ui.setLoadState(false, "Replay loaded: "+replay.parsedReplay.Username, 1)
			return
		}
	}
	ui.setLoadState(false, "This replay references a map that is not in the library.", 0)
}

func (ui *gioLauncherUI) loadLatestReplay() {
	dir := ui.l.currentConfig.General.GetReplaysDir()
	entries, err := os.ReadDir(dir)
	if err != nil { return }
	var newest string
	var newestTime int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".osr") { continue }
		info, err := entry.Info()
		if err != nil { continue }
		if info.ModTime().UnixNano() > newestTime {
			newestTime = info.ModTime().UnixNano()
			newest = filepath.Join(dir, entry.Name())
		}
	}
	if newest != "" { ui.loadReplayFile(newest) }
}

func (ui *gioLauncherUI) launchGame() {
	if !ui.l.canStart() { ui.setLoadState(false, "Select a beatmap or replay first.", 0); return }
	if ui.processActive { return }

	executable := os.Args[0]
	if build.Stream == "Release" { executable = filepath.Join(env.LibDir(), build.TaktChaseExec) }
	if runtime.GOOS != "windows" {
		if stat, err := os.Stat(executable); err == nil {
			_ = os.Chmod(executable, stat.Mode().Perm()|0111)
		}
	}

	args := ui.l.bld.getArguments()
	cmd := exec.Command(executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		ui.setLoadState(false, "TaktChase failed to start: "+err.Error(), 0)
		return
	}

	ui.statusMu.Lock()
	ui.process = cmd
	ui.processActive = true
	ui.status = "TaktChase is running"
	ui.statusMu.Unlock()
	ui.l.taktchaseCmd = cmd
	ui.l.taktchaseRunning = true
	ui.window.Invalidate()

	go func() {
		err := cmd.Wait()
		ui.statusMu.Lock()
		ui.processActive = false
		ui.process = nil
		if err != nil { ui.status = "TaktChase exited: "+err.Error() } else { ui.status = "TaktChase closed" }
		ui.statusMu.Unlock()
		ui.l.taktchaseCmd = nil
		ui.l.taktchaseRunning = false
		ui.window.Invalidate()
	}()
}

func (ui *gioLauncherUI) stopGame() {
	ui.statusMu.RLock()
	cmd := ui.process
	ui.statusMu.RUnlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		ui.setLoadState(false, "Stopping TaktChase…", 0)
	}
}

func (ui *gioLauncherUI) saveAllSettings() {
	if ui.l.currentConfig != nil {
		settings.SaveCredentials(false)
		ui.l.currentConfig.Save("", false)
	}
	saveLauncherConfig()
	ui.rebuildSettingsFields()
	ui.setLoadState(false, "Settings saved.", 1)
}

