package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/wieku/rplpa"

	"github.com/shntsk/taktchase/app/beatmap"
	"github.com/shntsk/taktchase/app/settings"
)

type localReplayEntry struct {
	path      string
	username  string
	score     int32
	maxCombo  uint16
	countMiss uint16
	timestamp time.Time
	mods      string
}

// localReplaysFor loads only scores matching the currently selected difficulty.
// The short cache avoids repeatedly decompressing every .osr on each UI frame.
func (b *mapBrowser) localReplaysFor(m *beatmap.BeatMap) []localReplayEntry {
	if m == nil || strings.TrimSpace(m.MD5) == "" {
		return nil
	}
	if strings.EqualFold(b.localReplayMD5, m.MD5) && time.Since(b.localReplaysUpdated) < 3*time.Second {
		return b.localReplays
	}

	b.localReplayMD5 = m.MD5
	b.localReplaysUpdated = time.Now()
	b.localReplays = b.localReplays[:0]

	dir := filepath.Join(settings.General.GetReplaysDir(), "TaktChase")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".osr") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		replay, err := rplpa.ParseReplay(data)
		if err != nil || replay == nil || !strings.EqualFold(replay.BeatmapMD5, m.MD5) || len(replay.ReplayData) < 3 {
			continue
		}

		username := strings.TrimSpace(replay.Username)
		if username == "" {
			username = "Jogador local"
		}

		mods := "NM"
		if replay.Mods != 0 {
			mods = fmt.Sprintf("Mods: %08X", replay.Mods)
		}

		b.localReplays = append(b.localReplays, localReplayEntry{
			path:      path,
			username:  username,
			score:     replay.Score,
			maxCombo:  replay.MaxCombo,
			countMiss: replay.CountMiss,
			timestamp: replay.Timestamp,
			mods:      mods,
		})
	}

	sort.SliceStable(b.localReplays, func(i, j int) bool {
		if b.localReplays[i].score != b.localReplays[j].score {
			return b.localReplays[i].score > b.localReplays[j].score
		}
		return b.localReplays[i].timestamp.After(b.localReplays[j].timestamp)
	})

	return b.localReplays
}

func (b *mapBrowser) drawLocalRanking(m *beatmap.BeatMap, iw, ip, contentY float32) float32 {
	entries := b.localReplaysFor(m)
	startY := contentY + 40
	rowH := float32(62)

	imgui.SetCursorPos(vec2(ip, contentY+3))
	imgui.PushFont(Font, 14)
	textColored("Melhores jogadas neste computador", cMuted)
	imgui.PopFont()

	if len(entries) == 0 {
		cardY := startY
		imgui.SetCursorPos(vec2(ip, cardY))
		p := imgui.CursorScreenPos()
		fillRound(p, p.Add(vec2(iw, 82)), cCard, 12)
		strokeRound(p, p.Add(vec2(iw, 82)), withAlpha(cText, 0.06), 12, 1)
		imgui.SetCursorPos(vec2(ip+14, cardY+13))
		imgui.PushFont(Font, 15)
		textColored("Nenhuma jogada salva ainda.", cText)
		imgui.PopFont()
		imgui.SetCursorPos(vec2(ip+14, cardY+39))
		imgui.PushFont(Font, 13)
		textColored("Jogue este mapa; o replay aparecerá aqui automaticamente.", cMuted)
		imgui.PopFont()
		imgui.SetCursorPos(vec2(ip, cardY+90))
		imgui.Dummy(vec2(iw, 1))
		return imgui.CursorPosY()
	}

	for i, replay := range entries {
		y := startY + float32(i)*rowH
		imgui.SetCursorPos(vec2(ip, y))
		p := imgui.CursorScreenPos()
		fillRound(p, p.Add(vec2(iw, rowH-5)), cCard, 9)
		if i == 0 {
			strokeRound(p, p.Add(vec2(iw, rowH-5)), withAlpha(cAccent, 0.38), 9, 1)
		}

		imgui.SetCursorPos(vec2(ip+10, y+7))
		imgui.PushFont(Font, 14)
		textColored(fmt.Sprintf("#%d  %s", i+1, b.truncate(replay.username, iw-132, 14)), cText)
		imgui.PopFont()

		stamp := "data desconhecida"
		if !replay.timestamp.IsZero() {
			stamp = replay.timestamp.Local().Format("02/01/2006 15:04")
		}
		imgui.SetCursorPos(vec2(ip+10, y+31))
		imgui.PushFont(Font, 12)
		textColored(fmt.Sprintf("%s  •  %d combo  •  %d misses  •  %s  •  %s", replayScore(replay.score), replay.maxCombo, replay.countMiss, stamp, replay.mods), cMuted)
		imgui.PopFont()

		buttonW := float32(84)
		buttonY := y + 15
		pushButtonStyle(cAccent, cAccentHov, cAccentAct, 8)
		imgui.PushFont(Font, 13)
		imgui.SetCursorPos(vec2(ip+iw-buttonW-8, buttonY))
		if imgui.ButtonV("Assistir##localReplay"+fmt.Sprint(i), vec2(buttonW, 29)) {
			b.l.trySelectReplayFromPath(replay.path)
			if b.l.bld.replayPath == replay.path && b.l.canStart() {
				launcherConfig.CurrentPMode = Watch
				b.l.startOrCancel(false)
			}
		}
		imgui.PopFont()
		popButtonStyle()
	}

	endY := startY + float32(len(entries))*rowH + 4
	imgui.SetCursorPos(vec2(ip, endY))
	imgui.Dummy(vec2(iw, 1))
	return imgui.CursorPosY()
}

func replayScore(score int32) string {
	return fmt.Sprintf("%d", score)
}
