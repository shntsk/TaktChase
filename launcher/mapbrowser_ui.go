package launcher

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/shntsk/taktchase/app/beatmap"
	"github.com/shntsk/taktchase/framework/platform"
)

// ---------------------------------------------------------------- theme

var (
	cPanel     = vec4(0.09, 0.09, 0.10, 0.93)
	cCard      = vec4(0.13, 0.13, 0.14, 0.92)
	cCardHover = vec4(0.17, 0.17, 0.19, 0.95)
	cCardSel   = vec4(0.19, 0.19, 0.21, 0.98)
	cDiffRow   = vec4(0.11, 0.11, 0.12, 0.92)
	cField     = vec4(0.11, 0.11, 0.12, 0.95)
	cFieldHov  = vec4(0.16, 0.16, 0.18, 0.95)
	cBtn       = vec4(0.17, 0.17, 0.19, 1)
	cBtnHov    = vec4(0.23, 0.23, 0.26, 1)
	cBtnAct    = vec4(0.28, 0.28, 0.31, 1)
	cAccent    = vec4(0.22, 0.78, 0.84, 1)
	cAccentHov = vec4(0.35, 0.86, 0.90, 1)
	cAccentAct = vec4(0.12, 0.66, 0.74, 1)
	cText      = vec4(1, 1, 1, 1)
	cMuted     = vec4(0.64, 0.64, 0.68, 1)
	cDim       = vec4(0.50, 0.50, 0.54, 1)
	cTrack     = vec4(0.22, 0.22, 0.24, 1)
	cInk       = vec4(0.05, 0.07, 0.08, 1)
	cClear     = vec4(0, 0, 0, 0)
)

func scaleCol(c imgui.Vec4, f float32) imgui.Vec4 {
	return vec4(c.X*f, c.Y*f, c.Z*f, c.W)
}

func withAlpha(c imgui.Vec4, a float32) imgui.Vec4 {
	return vec4(c.X, c.Y, c.Z, a)
}

// NOTE: these two helpers are the only place where the ImDrawList rounded-rect API is used.
func fillRound(p0, p1 imgui.Vec2, c imgui.Vec4, rounding float32) {
	imgui.WindowDrawList().AddRectFilledV(p0, p1, packColor(c), rounding, imgui.DrawFlagsRoundCornersAll)
}

func strokeRound(p0, p1 imgui.Vec2, c imgui.Vec4, rounding, thickness float32) {
	imgui.WindowDrawList().AddRectV(p0, p1, packColor(c), rounding, imgui.DrawFlagsRoundCornersAll, thickness)
}

func textColored(s string, c imgui.Vec4) {
	imgui.PushStyleColorVec4(imgui.ColText, c)
	imgui.TextUnformatted(s)
	imgui.PopStyleColor()
}

func textWidth(s string) float32 {
	return imgui.CalcTextSizeV(s, false, 0).X
}

func clampF(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}

// pushButtonStyle / popButtonStyle give rounded, borderless buttons in the launcher theme.
func pushButtonStyle(bg, hov, act imgui.Vec4, rounding float32) {
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, rounding)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColButton, bg)
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, hov)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, act)
}

func popColors(n int) {
	for i := 0; i < n; i++ {
		imgui.PopStyleColor()
	}
}

func popVars(n int) {
	for i := 0; i < n; i++ {
		imgui.PopStyleVar()
	}
}

func setCursorX(x float32) {
	imgui.SetCursorPos(vec2(x, imgui.CursorPos().Y))
}

func popButtonStyle() {
	popColors(3)
	popVars(2)
}

func pushFieldStyle() {
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 12)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, vec2(14, 9))
	imgui.PushStyleColorVec4(imgui.ColFrameBg, cField)
	imgui.PushStyleColorVec4(imgui.ColFrameBgHovered, cFieldHov)
	imgui.PushStyleColorVec4(imgui.ColFrameBgActive, cFieldHov)
	imgui.PushStyleColorVec4(imgui.ColButton, cField)
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, cFieldHov)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, cBtnAct)
}

func popFieldStyle() {
	popColors(6)
	popVars(3)
}

// ---------------------------------------------------------------- screen

func (l *launcher) drawBrowserScreen() {
	if l.selectWindow == nil {
		l.selectWindow = newSongSelectPopup(l.bld, l.beatmaps)
	}

	if l.browser == nil {
		l.browser = newMapBrowser(l, l.selectWindow)
	}

	l.browser.draw()
}

func (b *mapBrowser) draw() {
	b.thumbs.pump()

	// The selection can change from outside (replay loaded, .osz dropped, database reloaded).
	if cur := b.l.bld.currentMap; cur != b.lastCur {
		if cur != nil && (b.lastCur == nil || b.lastCur.Dir != cur.Dir) {
			b.expandedDir = cur.Dir
			b.layoutDirty = true
			b.scrollFrames = 3
		}

		b.lastCur = cur
	}

	if b.sel.focusTheMap { // set by selectNewest()/setBeatmaps()
		b.sel.focusTheMap = false
		b.centerNext = true
	}

	if b.sel.browserDirty {
		b.needRebuild = true
	}

	if b.needRebuild {
		b.rebuild()
	}

	if b.layoutDirty {
		b.relayout()
	}

	b.updatePreview()
	b.handleKeys()

	win := imgui.WindowSize()
	W, H := win.X, win.Y

	const (
		pad      = 18
		headerHt = 92
		footerHt = 82
		gap      = 16
	)

	cfgW := clampF(W*0.34, 330, 460)

	b.drawHeader(W, cfgW)

	panelH := H - headerHt - footerHt
	leftW := clampF(W*0.48, 330, 730)
	rightX := pad + leftW + gap
	rightW := max(W-rightX-pad, 200)

	imgui.SetCursorPos(vec2(pad, headerHt))
	b.drawLeft(leftW, panelH)

	imgui.SetCursorPos(vec2(rightX, headerHt))
	b.drawRight(rightW, panelH)

	b.drawFooter(W, H)
}

func (b *mapBrowser) drawHeader(W, cfgW float32) {
	l := b.l
	imgui.SetCursorPos(vec2(0, 0))
	hp := imgui.CursorScreenPos()
	fillRound(hp, hp.Add(vec2(W, 92)), withAlpha(cPanel, 0.98), 0)
	fillRound(hp.Add(vec2(0, 91)), hp.Add(vec2(W, 92)), withAlpha(cText, 0.08), 0)

	imgui.SetCursorPos(vec2(24, 10))

	imgui.PushFont(Font, 38)
	title := "TaktChase"
	imgui.TextUnformatted(title)
	tw := textWidth(title)
	imgui.PopFont()

	imgui.PushFont(Font, 18)

	var info string

	switch {
	case len(b.sel.beatmaps) == 0:
		info = "nenhum mapa"
	case len(b.filters) > 0 || len(b.terms) > 0:
		info = fmt.Sprintf("%d de %d conjuntos, %d dificuldades", len(b.sets), b.totalSetsAll, b.totalDiffs)
	default:
		info = fmt.Sprintf("%d conjuntos, %d dificuldades", len(b.sets), b.totalDiffs)
	}

	imgui.SetCursorPos(vec2(24+tw+14, 28))
	textColored(info, cMuted)

	imgui.PopFont()

	imgui.SetCursorPos(vec2(24, 58))
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Modo:")
	imgui.SameLine()
	imgui.SetNextItemWidth(clampF(W*0.2, 190, 260))

	pushFieldStyle()
	l.drawModeCombo()
	popFieldStyle()

	l.drawConfigPanelAt(vec2(W-24-cfgW, 10), cfgW)
}

func (b *mapBrowser) drawFooter(W, H float32) {
	const footerH = float32(82)
	startY := H - footerH
	imgui.SetCursorPos(vec2(0, startY))
	p := imgui.CursorScreenPos()
	fillRound(p, p.Add(vec2(W, footerH)), vec4(0.055, 0.06, 0.07, 0.98), 0)
	fillRound(p, p.Add(vec2(W, 1)), withAlpha(cText, 0.10), 0)

	cur := b.l.bld.currentMap
	btnH := float32(54)
	btnY := startY + 12
	sp := float32(10)
	speedW, timeW := float32(76), float32(76)
	modsW, randomW, optionsW := float32(112), float32(132), float32(132)
	previewW, linkW, playW := float32(50), float32(50), float32(150)
	groupW := speedW + timeW + modsW + randomW + optionsW + previewW + linkW + playW + 7*sp
	x := max(float32(16), (W-groupW)/2)

	pushButtonStyle(cBtn, cBtnHov, cBtnAct, 13)
	imgui.PushFont(Font, 18)
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV("Speed##footerSpeed", vec2(speedW, btnH)) {
		b.l.openSpeedPopup()
	}
	x += speedW + sp
	if cur == nil {
		imgui.BeginDisabled()
	}
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV("Time##footerTime", vec2(timeW, btnH)) {
		b.l.openTimePopup()
	}
	if cur == nil {
		imgui.EndDisabled()
	}
	x += timeW + sp
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV("\u21c4  Mods", vec2(modsW, btnH)) {
		b.l.openModsPopup()
	}
	x += modsW + sp
	imgui.SetCursorPos(vec2(x, btnY))
	noSets := len(b.sets) == 0
	if noSets {
		imgui.BeginDisabled()
	}
	if imgui.ButtonV("\u21c6  Aleatório", vec2(randomW, btnH)) {
		b.selectRandom()
	}
	if noSets {
		imgui.EndDisabled()
	}
	x += randomW + sp
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV("\u2699  Opções", vec2(optionsW, btnH)) {
		b.l.openLauncherSettings()
	}
	imgui.PopFont()

	x += optionsW + sp
	playing := cur != nil && b.sel.PreviewedSong != nil && b.sel.prevMap != nil && b.sel.prevMap.Dir == cur.Dir
	previewIcon := "\u25b6"
	if playing {
		previewIcon = "\u25a0"
	}
	if cur == nil {
		imgui.BeginDisabled()
	}
	imgui.PushFont(FontAw, 18)
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV(previewIcon+"##footerPreview", vec2(previewW, btnH)) {
		b.togglePreview()
	}
	imgui.PopFont()
	if cur == nil {
		imgui.EndDisabled()
	}
	x += previewW + sp
	noLink := cur == nil || cur.SetID == 0
	if noLink {
		imgui.BeginDisabled()
	}
	imgui.PushFont(FontAw, 18)
	imgui.SetCursorPos(vec2(x, btnY))
	if imgui.ButtonV("\uF7A2##footerLink", vec2(linkW, btnH)) {
		platform.OpenURL(fmt.Sprintf("https://osu.ppy.sh/s/%d", cur.SetID))
	}
	imgui.PopFont()
	if noLink {
		imgui.EndDisabled()
	}
	x += linkW + sp
	label := "Jogar"
	if launcherConfig.CurrentMode != Play {
		switch launcherConfig.CurrentPMode {
		case Record:
			label = "Gravar"
		case Screenshot:
			label = "Capturar"
		}
	}
	popButtonStyle()
	pushButtonStyle(cAccent, cAccentHov, cAccentAct, 13)
	imgui.PushFont(Font, 21)
	imgui.SetCursorPos(vec2(x, btnY))
	b.l.drawStartButton(label, vec2(playW, btnH))
	imgui.PopFont()
	popButtonStyle()
}

// ---------------------------------------------------------------- keyboard

func (b *mapBrowser) handleKeys() {
	l := b.l

	enter := imgui.IsKeyDown(imgui.KeyEnter) || imgui.IsKeyDown(imgui.KeyKeypadEnter)

	if l.taktchaseRunning || len(l.popupStack) > 0 || l.newCloneOpened || l.configEditOpened || (imgui.IsAnyItemActive() && !b.searchActive) {
		b.enterDown = enter

		return
	}

	ctrl := imgui.IsKeyDown(imgui.KeyLeftCtrl) || imgui.IsKeyDown(imgui.KeyRightCtrl)

	if (ctrl && imgui.IsKeyPressedBool(imgui.KeyF)) || (!b.searchActive && imgui.IsKeyPressedBool(imgui.KeyTab)) {
		b.focusSearch = true
	}

	if imgui.IsKeyPressedBool(imgui.KeyEscape) && (b.searchActive || b.searchStr != "") {
		b.setSearch("")
	}

	if imgui.IsKeyPressedBool(imgui.KeyUpArrow) {
		b.moveSet(-1)
	}

	if imgui.IsKeyPressedBool(imgui.KeyDownArrow) {
		b.moveSet(1)
	}

	if imgui.IsKeyPressedBool(imgui.KeyPageUp) {
		b.moveSet(-6)
	}

	if imgui.IsKeyPressedBool(imgui.KeyPageDown) {
		b.moveSet(6)
	}

	if !b.searchActive { // the text field owns these while it is being edited
		if imgui.IsKeyPressedBool(imgui.KeyLeftArrow) {
			b.moveDiff(-1)
		}

		if imgui.IsKeyPressedBool(imgui.KeyRightArrow) {
			b.moveDiff(1)
		}

		if imgui.IsKeyPressedBool(imgui.KeyHome) {
			b.selectEdge(false)
		}

		if imgui.IsKeyPressedBool(imgui.KeyEnd) {
			b.selectEdge(true)
		}
	}

	if imgui.IsKeyPressedBool(imgui.KeyF2) {
		b.selectRandom()
	}

	if imgui.IsKeyPressedBool(imgui.KeyF3) {
		l.openLauncherSettings()
	}

	if enter && !b.enterDown && l.canStart() {
		l.startOrCancel(false)
	}

	b.enterDown = enter
}

// ---------------------------------------------------------------- left panel

func (b *mapBrowser) beginPanel(id string, w, h float32) bool {
	imgui.PushStyleColorVec4(imgui.ColChildBg, cPanel)
	imgui.PushStyleVarFloat(imgui.StyleVarChildRounding, 16)

	ok := imgui.BeginChildStrV(id, vec2(w, h), imgui.ChildFlagsNone, 0)

	imgui.PopStyleVar()
	imgui.PopStyleColor()

	return ok
}

func (b *mapBrowser) drawLeft(w, h float32) {
	l := b.l
	cur := l.bld.currentMap

	if b.beginPanel("##leftpanel", w, h) {
		const ip = 18

		iw := w - 2*ip
		frameH := imgui.FrameHeight()

		if cur == nil {
			b.drawLeftEmpty(w, h)
		} else {
			endY := b.drawMapInfo(cur, iw, ip)

			// The tabs scroll independently while the main actions remain pinned below.
			ctlH := float32(0)
			if launcherConfig.CurrentMode != Play {
				ctlH = frameH + 20
				if launcherConfig.CurrentPMode == Record && l.showProgressBar {
					ctlH += frameH * 2
				}
			}

			ctlY := h - ip - ctlH
			tabsY := endY + 10
			tabsH := max(float32(72), ctlY-tabsY-8)
			imgui.SetCursorPos(vec2(ip, tabsY))
			imgui.PushStyleColorVec4(imgui.ColChildBg, cClear)
			if imgui.BeginChildStrV("##mapinfotabs", vec2(iw, tabsH), imgui.ChildFlagsNone, 0) {
				b.drawInfoTabs(cur, iw, 0, 0)
			}
			imgui.EndChild()
			imgui.PopStyleColor()

			if launcherConfig.CurrentMode != Play {
				b.drawControls(iw, ip, ctlY)
			}
		}

	}

	imgui.EndChild()
}

func (b *mapBrowser) drawLeftEmpty(w, h float32) {
	imgui.PushFont(Font, 24)

	msg := "Nenhum mapa selecionado"
	imgui.SetCursorPos(vec2((w-textWidth(msg))/2, h*0.32))
	textColored(msg, cText)

	imgui.PopFont()

	imgui.PushFont(Font, 17)

	sub := "Escolha um mapa na lista à direita"
	imgui.SetCursorPos(vec2((w-textWidth(sub))/2, h*0.32+34))
	textColored(sub, cMuted)

	imgui.PopFont()
}

// drawMapInfo draws the selected beatmap title, difficulty and stats. It returns the ending Y position.
func (b *mapBrowser) drawMapInfo(m *beatmap.BeatMap, iw, ip float32) float32 {
	y := float32(ip)
	// The selected map is presented as an overlay, leaving the beatmap artwork
	// visible behind the panel instead of adding a second, oversized cover.
	y += 2

	// names
	imgui.SetCursorPos(vec2(ip, y))

	imgui.PushFont(Font, 26)
	textColored(b.truncate(m.Name, iw, 26), vec4(1, 0.78, 0.22, 1))

	if imgui.IsItemHovered() {
		setTooltip(m.Name)
	}

	imgui.PopFont()

	imgui.PushFont(Font, 19)
	setCursorX(ip)
	textColored(b.truncate(m.Artist, iw, 19), cMuted)
	imgui.PopFont()

	imgui.PushFont(Font, 15)
	setCursorX(ip)
	textColored(b.truncate("mapeado por "+m.Creator, iw, 15), cDim)
	imgui.PopFont()

	y = imgui.CursorPosY() + 10

	// difficulty chip + stars
	dc := diffColor(m.Stars)

	imgui.PushFont(Font, 17)

	name := b.truncate(m.Difficulty, iw-110, 17)
	chipW := textWidth(name) + 26
	chipH := float32(28)

	chipP := vec2(ip, y)

	imgui.SetCursorPos(chipP)
	cp := imgui.CursorScreenPos()
	fillRound(cp, cp.Add(vec2(chipW, chipH)), scaleCol(dc, 0.32), 14)
	imgui.Dummy(vec2(chipW, chipH))

	imgui.SetCursorPos(vec2(ip+13, y+4))
	imgui.TextUnformatted(name)

	imgui.PopFont()

	imgui.PushFont(FontAw, 17)
	imgui.SetCursorPos(vec2(ip+chipW+14, y+5))
	textColored("\uF005", dc)
	imgui.PopFont()

	imgui.PushFont(Font, 24)
	imgui.SetCursorPos(vec2(ip+chipW+38, y+1))
	textColored(fmtStars(m.Stars), dc)
	imgui.PopFont()

	y += chipH + 12

	// star bar
	imgui.SetCursorPos(vec2(ip, y))
	b.drawBar(float32(max(m.Stars, 0)/10), iw, 6, dc)

	imgui.PushFont(Font, 14)
	setCursorX(ip)
	textColored("classificação de estrelas", cDim)
	imgui.PopFont()

	y = imgui.CursorPosY() + 8

	// CS / AR / OD / HP in a single compact row, like osu!lazer's map panel.
	stats := []struct {
		name string
		val  float64
	}{
		{"CS", m.Diff.GetBaseCS()},
		{"AR", m.Diff.GetBaseAR()},
		{"OD", m.Diff.GetBaseOD()},
		{"HP", m.Diff.GetBaseHP()},
	}

	statGap := float32(10)
	statW := (iw - statGap*3) / 4
	for i, s := range stats {
		sx := ip + float32(i)*(statW+statGap)
		imgui.PushFont(Font, 13)
		imgui.SetCursorPos(vec2(sx, y))
		textColored(s.name, cMuted)
		imgui.PopFont()
		v := fmtStat(s.val)
		imgui.PushFont(Font, 15)
		imgui.SetCursorPos(vec2(sx, y+16))
		imgui.TextUnformatted(v)
		imgui.PopFont()
		imgui.SetCursorPos(vec2(sx, y+37))
		b.drawBar(float32(s.val)/10, statW, 4, cAccent)
	}
	y += 48

	// bpm / length / objects
	objects := m.Circles + m.Sliders + m.Spinners

	imgui.PushFont(Font, 17)
	imgui.SetCursorPos(vec2(ip, y))
	imgui.TextUnformatted(b.truncate(fmt.Sprintf("BPM %s     %s     %d objetos", fmtBPM(m.MinBPM, m.MaxBPM), fmtDuration(m.Length), objects), iw, 17))

	imgui.PopFont()

	imgui.PushFont(Font, 15)
	setCursorX(ip)
	textColored(b.truncate(fmt.Sprintf("%d círculos    %d sliders    %d spinners", m.Circles, m.Sliders, m.Spinners), iw, 15), cDim)
	imgui.PopFont()

	return imgui.CursorPosY()
}

// drawInfoTabs mirrors osu!lazer's Details / Ranking switch. Ranking data is
// intentionally local-only until the project has an online score provider.
func (b *mapBrowser) drawInfoTabs(m *beatmap.BeatMap, iw, ip, y float32) float32 {
	const tabH = float32(32)
	imgui.SetCursorPos(vec2(ip, y))

	tabs := []struct {
		label string
		value int
	}{
		{"Detalhes", 0},
		{"Ranking", 1},
	}

	for i, tab := range tabs {
		if i > 0 {
			imgui.SameLine()
		}
		bg := cCard
		if b.infoTab == tab.value {
			bg = cBtnAct
		}
		pushButtonStyle(bg, cBtnHov, cBtnAct, 8)
		imgui.PushFont(Font, 16)
		if imgui.ButtonV(tab.label+"##mapInfoTab", vec2(94, tabH)) {
			b.infoTab = tab.value
		}
		imgui.PopFont()
		popButtonStyle()
	}

	contentY := y + tabH + 9
	if b.infoTab == 0 {
		colGap := float32(12)
		colW := (iw - colGap*2) / 3
		fields := []struct {
			label string
			value string
		}{
			{"Criador", valueOrDash(m.Creator)},
			{"Fonte", valueOrDash(m.Source)},
			{"Modo", "osu!standard"},
			{"Artista", valueOrDash(m.Artist)},
			{"Enviado", "Data não disponível"},
			{"ID do mapa", strconv.FormatInt(m.ID, 10)},
		}

		imgui.PushFont(Font, 13)
		for i, field := range fields {
			col := i % 3
			row := i / 3
			x := ip + float32(col)*(colW+colGap)
			fy := contentY + float32(row)*42
			imgui.SetCursorPos(vec2(x, fy))
			textColored(field.label, cMuted)
			imgui.SetCursorPos(vec2(x, fy+18))
			imgui.PushFont(Font, 15)
			textColored(b.truncate(field.value, colW, 15), cText)
			imgui.PopFont()
		}
		imgui.PopFont()

		tagsY := contentY + 88
		imgui.SetCursorPos(vec2(ip, tagsY))
		imgui.PushFont(Font, 13)
		textColored("Tags", cMuted)
		imgui.PopFont()
		tags := strings.TrimSpace(m.Tags)
		if tags == "" {
			tags = "Nenhuma tag cadastrada"
		}
		imgui.SetCursorPos(vec2(ip, tagsY+18))
		imgui.PushFont(Font, 14)
		textColored(b.truncate(tags, iw, 14), cText)
		imgui.PopFont()

		statsY := tagsY + 42
		imgui.SetCursorPos(vec2(ip, statsY))
		fillRound(imgui.CursorScreenPos(), imgui.CursorScreenPos().Add(vec2(iw, 52)), cCard, 10)
		imgui.Dummy(vec2(iw, 52))
		items := []struct{ title, value string }{
			{"Taxa de sucesso", "—"},
			{"Avaliações", "—"},
			{"Gráfico de avaliações", "Ainda sem dados"},
		}
		for i, item := range items {
			x := ip + float32(i)*iw/3 + 10
			imgui.SetCursorPos(vec2(x, statsY+10))
			imgui.PushFont(Font, 12)
			textColored(item.title, cMuted)
			imgui.PopFont()
			imgui.SetCursorPos(vec2(x, statsY+29))
			imgui.PushFont(Font, 15)
			textColored(item.value, cText)
			imgui.PopFont()
		}
		imgui.SetCursorPos(vec2(ip, statsY+57))
		imgui.Dummy(vec2(iw, 1))
		return imgui.CursorPosY()
	}

	// Ranking scope selector. Local is the default, matching the reference.
	scopes := []string{"Global", "Local", "País", "Amigos"}
	scopeW := (iw - 3*5) / 4
	for i, scope := range scopes {
		if i > 0 {
			imgui.SameLine()
		}
		bg := cCard
		if b.rankingScope == i {
			bg = cBtnAct
		}
		pushButtonStyle(bg, cBtnHov, cBtnAct, 8)
		imgui.PushFont(Font, 14)
		if imgui.ButtonV(scope+"##rankingScope", vec2(scopeW, 30)) {
			b.rankingScope = i
		}
		imgui.PopFont()
		popButtonStyle()
	}

	// Local is backed by replays created by actual TaktChase plays. The other
	// scopes stay as explicit placeholders until an online score source exists.
	if b.rankingScope == 1 {
		return b.drawLocalRanking(m, iw, ip, contentY)
	}

	cardY := contentY + 42
	imgui.SetCursorPos(vec2(ip, cardY))
	cardP := imgui.CursorScreenPos()
	fillRound(cardP, cardP.Add(vec2(iw, 86)), cCard, 12)
	strokeRound(cardP, cardP.Add(vec2(iw, 86)), withAlpha(cText, 0.06), 12, 1)

	titles := []string{"Ranking global", "Ranking local", "Ranking do país", "Ranking de amigos"}
	imgui.SetCursorPos(vec2(ip+14, cardY+12))
	imgui.PushFont(Font, 16)
	textColored(titles[b.rankingScope], cText)
	imgui.PopFont()
	imgui.SetCursorPos(vec2(ip+14, cardY+39))
	imgui.PushFont(Font, 14)
	textColored("Nenhuma pontuação disponível ainda.", cMuted)
	imgui.PopFont()
	imgui.SetCursorPos(vec2(ip+14, cardY+60))
	imgui.PushFont(Font, 12)
	textColored("Estrutura preparada para dados locais e suporte online futuro.", cDim)
	imgui.PopFont()
	imgui.SetCursorPos(vec2(ip, cardY+92))
	imgui.Dummy(vec2(iw, 1))
	return imgui.CursorPosY()
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func (b *mapBrowser) drawBar(frac, w, h float32, c imgui.Vec4) {
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 3)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColPlotHistogram, c)
	imgui.PushStyleColorVec4(imgui.ColFrameBg, cTrack)

	imgui.ProgressBarV(clampF(frac, 0, 1), vec2(w, h), "")

	popColors(2)
	popVars(2)
}

// drawControls keeps the non-play mode controls in the map panel. Common
// actions (Speed, Mods, Random, Options and Play) live in the pinned footer.
func (b *mapBrowser) drawControls(iw, ip, y float32) {
	l := b.l
	if launcherConfig.CurrentMode != Play {
		imgui.SetCursorPos(vec2(ip, y+10))

		pushFieldStyle()
		l.drawPModeBody(iw*0.45, iw, launcherConfig.CurrentPMode == Record && l.showProgressBar)
		popFieldStyle()
	}
}

// ---------------------------------------------------------------- right panel

func (b *mapBrowser) drawRight(w, h float32) {
	x0 := imgui.CursorPos().X
	y0 := imgui.CursorPos().Y

	const tbH = 44

	// toolbar
	pushFieldStyle()

	narrow := w < 620

	sortW := float32(180)
	groupW := float32(180)

	if narrow {
		sortW, groupW = 130, 130
	}

	ascW := float32(44)
	searchW := max(w-sortW-groupW-ascW-3*8, 120)

	if b.focusSearch {
		imgui.SetKeyboardFocusHereV(0)
		b.focusSearch = false
	}

	imgui.SetNextItemWidth(searchW)

	prev := b.searchStr
	imgui.InputTextWithHint("##browsersearch", "Procurar... (ex.: stars>5 bpm>180)", &b.searchStr, imgui.InputTextFlagsNone, nil)
	b.searchActive = imgui.IsItemActive()

	if b.searchStr != prev {
		b.needRebuild = true
		b.centerNext = true
	}

	if imgui.IsItemHovered() {
		imgui.SetTooltip("Busca por título, artista, mapper e dificuldade.\nFiltros: stars, bpm, cs, ar, od, hp, length (s), objects\nUse > >= < <= =, por exemplo: stars>5 bpm>=180")
	}

	imgui.SameLine()
	imgui.SetNextItemWidth(sortW)

	if imgui.BeginCombo("##sortby", "Ordenar: "+launcherConfig.SortMapsBy.String()) {
		for _, s := range sortMethods {
			if imgui.SelectableBoolV(s.String(), s == launcherConfig.SortMapsBy, 0, vzero()) && s != launcherConfig.SortMapsBy {
				launcherConfig.SortMapsBy = s
				b.needRebuild = true
				b.centerNext = true

				saveLauncherConfig()
			}
		}

		imgui.EndCombo()
	}

	imgui.SameLine()

	imgui.PushFont(FontAw, 18)

	sDir := "\uF882"
	if launcherConfig.SortAscending {
		sDir = "\uF15D"
	}

	if imgui.ButtonV(sDir+"##sortdir", vec2(ascW, 0)) {
		launcherConfig.SortAscending = !launcherConfig.SortAscending
		b.needRebuild = true
		b.centerNext = true

		saveLauncherConfig()
	}

	imgui.PopFont()

	if imgui.IsItemHovered() {
		if launcherConfig.SortAscending {
			imgui.SetTooltip("Crescente")
		} else {
			imgui.SetTooltip("Decrescente")
		}
	}

	imgui.SameLine()
	imgui.SetNextItemWidth(groupW)

	if imgui.BeginCombo("##groupby", "Grupo: "+GroupBy(launcherConfig.GroupMapsBy).String()) {
		for _, g := range groupMethods {
			if imgui.SelectableBoolV(g.String(), g == GroupBy(launcherConfig.GroupMapsBy), 0, vzero()) && g != GroupBy(launcherConfig.GroupMapsBy) {
				launcherConfig.GroupMapsBy = int(g)
				b.needRebuild = true
				b.centerNext = true

				saveLauncherConfig()
			}
		}

		imgui.EndCombo()
	}

	popFieldStyle()

	// list
	listY := y0 + tbH + 10

	imgui.SetCursorPos(vec2(x0, listY))

	imgui.PushStyleColorVec4(imgui.ColChildBg, cClear)
	imgui.PushStyleVarFloat(imgui.StyleVarScrollbarSize, 8)

	ok := imgui.BeginChildStrV("##maplist", vec2(w, h-tbH-10), imgui.ChildFlagsNone, 0)

	imgui.PopStyleVar()
	imgui.PopStyleColor()

	if ok {
		b.drawList()
	}

	imgui.EndChild()
}

func (b *mapBrowser) drawList() {
	handleDragScroll()

	avail := imgui.ContentRegionAvail()
	cw := avail.X
	viewH := avail.Y

	if len(b.sets) == 0 {
		b.drawListEmpty(cw, viewH)

		return
	}

	sy := imgui.ScrollY()
	cur := b.l.bld.currentMap

	if cur != nil {
		if ri, ok := b.rowIdx[cur.Dir]; ok {
			r := b.rows[ri]

			switch {
			case b.centerNext:
				sy = max(r.y-viewH*0.3, 0)
				imgui.SetScrollYFloat(sy)

				b.centerNext = false
				b.scrollFrames = 0
			case b.scrollFrames > 0:
				top, bot := r.y, r.y+cardH

				if r.set.dir == b.expandedDir {
					if di := slices.Index(r.set.maps, cur); di >= 0 {
						bot = r.y + cardH + diffPad + float32(di+1)*(diffH+diffGap) - diffGap
					}
				}

				const margin = 16

				ns := sy

				if top < sy+margin {
					ns = max(top-margin, 0)
				} else if bot > sy+viewH-margin {
					ns = max(min(bot-viewH+margin, top-margin), 0)
				}

				if ns != sy {
					sy = ns
					imgui.SetScrollYFloat(sy)
				}

				b.scrollFrames--
			}
		}
	}

	// the selectables are only used for hit testing, everything is drawn manually
	imgui.PushStyleColorVec4(imgui.ColHeader, cClear)
	imgui.PushStyleColorVec4(imgui.ColHeaderHovered, cClear)
	imgui.PushStyleColorVec4(imgui.ColHeaderActive, cClear)

	first := sort.Search(len(b.rows), func(i int) bool { return b.rows[i].y+b.rows[i].h >= sy-4 })

	for i := first; i < len(b.rows) && b.rows[i].y <= sy+viewH+4; i++ {
		r := &b.rows[i]

		if r.set == nil {
			b.drawGroupHeader(r, cw)
		} else {
			b.drawSetRow(i, r, cw)
		}
	}

	popColors(3)

	// extend the scrollable area to the real content height
	imgui.SetCursorPos(vec2(0, b.totalH))
	imgui.Dummy(vec2(1, 1))
}

func (b *mapBrowser) drawGroupHeader(r *bRow, cw float32) {
	imgui.PushFont(Font, 16)

	imgui.SetCursorPos(vec2(8, r.y+6))
	textColored(b.truncate(r.header, cw-120, 16), cAccent)

	cnt := fmt.Sprintf("%d sets", r.count)
	if r.count == 1 {
		cnt = "1 set"
	}

	imgui.SetCursorPos(vec2(cw-textWidth(cnt)-14, r.y+6))
	textColored(cnt, cDim)

	imgui.PopFont()
}

func (b *mapBrowser) drawSetRow(i int, r *bRow, cw float32) {
	s := r.set
	cur := b.l.bld.currentMap
	selected := cur != nil && cur.Dir == s.dir
	first := s.maps[0]

	imgui.SetCursorPos(vec2(0, r.y))
	p := imgui.CursorScreenPos()

	clicked := imgui.SelectableBoolV("##set"+strconv.Itoa(i), false, 0, vec2(cw, cardH))
	hovered := imgui.IsItemHovered()

	bg := cCard
	if selected {
		bg = cCardSel
	} else if hovered {
		bg = cCardHover
	}

	fillRound(p, p.Add(vec2(cw, cardH)), bg, 14)

	if selected {
		strokeRound(p, p.Add(vec2(cw, cardH)), cAccent, 14, 2)
	}

	// cover
	const (
		tW = 120
		tH = 67
	)

	imgui.SetCursorPos(vec2(12, r.y+(cardH-tH)/2))

	if t := b.thumbs.get(coverPath(first)); t != nil {
		imgui.ImageWithBgV(*t.ref, vec2(tW, tH), vec2(0, 0), vec2(1, 1), imgui.Vec4{}, vec4(1, 1, 1, 1))
	} else {
		tp := imgui.CursorScreenPos()
		fillRound(tp, tp.Add(vec2(tW, tH)), vec4(0.17, 0.19, 0.24, 1), 8)
		imgui.Dummy(vec2(tW, tH))
	}

	textX := float32(12 + tW + 16)

	// right column: star range
	rightW := float32(0)

	if s.maxStars >= 0 {
		rng := fmt.Sprintf("%.1f", s.maxStars)
		if s.minStars != s.maxStars && fmt.Sprintf("%.1f", s.minStars) != rng {
			rng = fmt.Sprintf("%.1f - %.1f", s.minStars, s.maxStars)
		}

		imgui.PushFont(Font, 15)
		tw := textWidth(rng)

		imgui.SetCursorPos(vec2(cw-14-tw, r.y+11))
		textColored(rng, cMuted)
		imgui.PopFont()

		imgui.PushFont(FontAw, 12)
		imgui.SetCursorPos(vec2(cw-14-tw-16, r.y+13))
		textColored("\uF005", cMuted)
		imgui.PopFont()

		rightW = tw + 16 + 14
	}

	titleMax := cw - textX - max(rightW, 60) - 14

	imgui.PushFont(Font, 20)
	imgui.SetCursorPos(vec2(textX, r.y+10))
	imgui.TextUnformatted(b.truncate(first.Name, titleMax, 20))
	imgui.PopFont()

	imgui.PushFont(Font, 17)
	imgui.SetCursorPos(vec2(textX, r.y+35))
	textColored(b.truncate(first.Artist, cw-textX-14, 17), cMuted)
	imgui.PopFont()

	// difficulty dots
	const (
		dot     = 8
		dotStep = 13
		maxDots = 14
	)

	nDots := min(len(s.maps), maxDots)

	for k := 0; k < nDots; k++ {
		dp := p.Add(vec2(cw-14-float32(nDots-k)*dotStep+(dotStep-dot), cardH-23))
		fillRound(dp, dp.Add(vec2(dot, dot)), diffColor(s.maps[k].Stars), dot/2)
	}

	imgui.PushFont(Font, 15)
	imgui.SetCursorPos(vec2(textX, r.y+57))
	textColored(b.truncate("mapeado por "+first.Creator, cw-textX-14-float32(nDots)*dotStep-8, 15), cDim)
	imgui.PopFont()

	if clicked {
		b.toggleSet(s)
	}

	// difficulties
	if s.dir != b.expandedDir {
		return
	}

	dy := r.y + cardH + diffPad
	indent := float32(24)

	for j, m := range s.maps {
		b.drawDiffRow(i, j, m, indent, dy, cw-indent, cur == m)

		dy += diffH + diffGap
	}
}

func (b *mapBrowser) drawDiffRow(i, j int, m *beatmap.BeatMap, x, y, w float32, selected bool) {
	imgui.SetCursorPos(vec2(x, y))
	p := imgui.CursorScreenPos()

	clicked := imgui.SelectableBoolV("##diff"+strconv.Itoa(i)+"_"+strconv.Itoa(j), false, 0, vec2(w, diffH))
	hovered := imgui.IsItemHovered()

	dc := diffColor(m.Stars)

	bg := cDiffRow
	if selected {
		bg = cCardSel
	} else if hovered {
		bg = cCardHover
	}

	fillRound(p, p.Add(vec2(w, diffH)), bg, 12)

	if selected {
		strokeRound(p, p.Add(vec2(w, diffH)), dc, 12, 2)
	}

	// ring
	ringP := p.Add(vec2(14, (diffH-22)/2))
	strokeRound(ringP, ringP.Add(vec2(22, 22)), dc, 11, 3.5)

	pillW := float32(88)
	textX := float32(14 + 22 + 16)

	imgui.PushFont(Font, 18)
	imgui.SetCursorPos(vec2(x+textX, y+6))
	imgui.TextUnformatted(b.truncate(m.Difficulty, w-textX-pillW-30, 18))
	imgui.PopFont()

	objects := m.Circles + m.Sliders + m.Spinners

	details := fmt.Sprintf("CS %s   AR %s   OD %s   HP %s   %d objetos",
		fmtStat(m.Diff.GetBaseCS()), fmtStat(m.Diff.GetBaseAR()), fmtStat(m.Diff.GetBaseOD()), fmtStat(m.Diff.GetBaseHP()), objects)

	imgui.PushFont(Font, 14)
	imgui.SetCursorPos(vec2(x+textX, y+27))
	textColored(b.truncate(details, w-textX-pillW-30, 14), cDim)
	imgui.PopFont()

	// stars pill
	pp := p.Add(vec2(w-pillW-12, (diffH-28)/2))
	fillRound(pp, pp.Add(vec2(pillW, 28)), dc, 14)

	imgui.PushFont(FontAw, 12)
	imgui.SetCursorPos(vec2(x+w-pillW-12+14, y+(diffH-28)/2+8))
	textColored("\uF005", cInk)
	imgui.PopFont()

	imgui.PushFont(Font, 17)
	st := fmtStars(m.Stars)
	imgui.SetCursorPos(vec2(x+w-12-14-textWidth(st), y+(diffH-28)/2+4))
	textColored(st, cInk)
	imgui.PopFont()

	if clicked {
		b.selectMap(m)
	}
}

func (b *mapBrowser) drawListEmpty(cw, viewH float32) {
	l := b.l

	var title, body string

	clear := false

	switch {
	case len(b.sel.beatmaps) == 0:
		title = "Nenhum mapa encontrado"
		body = "O TaktChase não encontrou mapas na pasta Songs.\nConfira o diretório nas configurações do perfil\nou solte arquivos .osz nesta janela."
	default:
		title = "Nenhum resultado"
		body = "Nada corresponde a \"" + b.searchStr + "\".\nTente outra busca ou ajuste os filtros."
		clear = true
	}

	y := max(viewH*0.28, 20)

	imgui.PushFont(Font, 26)
	imgui.SetCursorPos(vec2((cw-textWidth(title))/2, y))
	textColored(title, cText)
	imgui.PopFont()

	y += 44

	imgui.PushFont(Font, 17)

	for _, line := range splitLines(body) {
		imgui.SetCursorPos(vec2(max((cw-textWidth(line))/2, 0), y))
		textColored(line, cMuted)

		y += 24
	}

	imgui.PopFont()

	y += 14

	label := "Abrir configurações do perfil"
	if clear {
		label = "Limpar busca"
	}

	imgui.PushFont(Font, 18)

	bw := textWidth(label) + 48

	imgui.SetCursorPos(vec2((cw-bw)/2, y))

	pushButtonStyle(cAccent, cAccentHov, cAccentAct, 12)

	if imgui.ButtonV(label+"##empty", vec2(bw, 40)) {
		if clear {
			b.setSearch("")
		} else {
			l.openCurrentSettingsEditor()
		}
	}

	popButtonStyle()
	imgui.PopFont()
}

func splitLines(s string) []string {
	var out []string

	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}

	return append(out, s[start:])
}
