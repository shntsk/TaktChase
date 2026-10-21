package dance

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wieku/rplpa"

	"github.com/shntsk/taktchase/app/beatmap/difficulty"
	"github.com/shntsk/taktchase/app/graphics"
	"github.com/shntsk/taktchase/app/settings"
)

const replaySampleInterval = 1000.0 / 60.0

// captureReplayFrame stores standard osu! replay frames while a real play is running.
// The first two frames establish the initial cursor position; gameplay input starts
// on subsequent frames so playback begins at the correct map time.
func (controller *PlayerController) captureReplayFrame(gameTime float64) {
	if !settings.PLAY || gameTime < 0 || controller.replaySaved || len(controller.cursors) == 0 {
		return
	}

	cursor := controller.cursors[0]
	if !controller.replaySampleStarted {
		controller.replaySampleStarted = true
		controller.replayLastTime = gameTime
		controller.replayFrames = append(controller.replayFrames, newReplayFrame(0, cursor, false))
		controller.replayFrames = append(controller.replayFrames, newReplayFrame(1, cursor, false))
		return
	}

	delta := gameTime - controller.replayLastTime
	if delta < replaySampleInterval {
		return
	}

	controller.replayFrames = append(controller.replayFrames, newReplayFrame(delta, cursor, true))
	controller.replayLastTime = gameTime
}

func newReplayFrame(delta float64, cursor *graphics.Cursor, includeInput bool) *rplpa.ReplayData {
	keys := &rplpa.KeyPressed{}
	if includeInput {
		keys.LeftClick = cursor.LeftButton
		keys.RightClick = cursor.RightButton
		keys.Key1 = cursor.LeftKey
		keys.Key2 = cursor.RightKey
		keys.Smoke = cursor.SmokeKey
	}

	return &rplpa.ReplayData{
		Time:       max(delta, 0),
		MouseX:     float64(cursor.Position.X),
		MouseY:     float64(cursor.Position.Y),
		KeyPressed: keys,
	}
}

// SaveReplay writes the current gameplay as an ordinary .osr file in the configured
// replay directory. It is safe to call from the player's update loop more than once.
func (controller *PlayerController) SaveReplay() error {
	if controller.replaySaved || !settings.PLAY || controller.bMap == nil || controller.ruleset == nil || len(controller.cursors) == 0 || len(controller.replayFrames) < 3 {
		return nil
	}

	controller.replaySaved = true

	cursor := controller.cursors[0]
	score := controller.ruleset.GetScore(cursor)
	now := time.Now()
	version, _ := strconv.Atoi(now.Format("20060102"))

	replay := rplpa.NewReplay()
	replay.PlayMode = rplpa.OSU
	replay.OsuVersion = int32(version)
	if controller.bMap.Diff.Mods.Active(difficulty.Lazer) {
		replay.OsuVersion = 30000000
	}
	replay.BeatmapMD5 = controller.bMap.MD5
	replay.Username = settings.Gameplay.PlayUsername
	replay.Timestamp = now
	replay.ReplayData = controller.replayFrames
	replay.ScoreID = -1
	replay.Count300 = uint16(min(uint64(score.Count300), uint64(65535)))
	replay.CountGeki = uint16(min(uint64(score.CountGeki), uint64(65535)))
	replay.Count100 = uint16(min(uint64(score.Count100), uint64(65535)))
	replay.CountKatu = uint16(min(uint64(score.CountKatu), uint64(65535)))
	replay.Count50 = uint16(min(uint64(score.Count50), uint64(65535)))
	replay.CountMiss = uint16(min(uint64(score.CountMiss), uint64(65535)))
	replay.Score = int32(max(int64(0), min(score.Score, int64(1<<31-1))))
	replay.MaxCombo = uint16(min(uint64(score.Combo), uint64(65535)))
	replay.Fullcombo = score.CountMiss == 0 && score.CountSB == 0
	replay.Mods = uint32(controller.bMap.Diff.Mods)

	if controller.bMap.Diff.Mods.Active(difficulty.Lazer) {
		mods := controller.bMap.Diff.Mods.ConvertToModInfoList()
		replay.ScoreInfo = &rplpa.ScoreInfo{}
		for i := range mods {
			mod := mods[i]
			replay.ScoreInfo.Mods = append(replay.ScoreInfo.Mods, &mod)
		}
	}

	dir := filepath.Join(settings.General.GetReplaysDir(), "TaktChase")
	if err := os.MkdirAll(dir, 0755); err != nil {
		controller.replaySaved = false
		return fmt.Errorf("create replay directory: %w", err)
	}

	name := strings.TrimSpace(controller.bMap.Artist + " - " + controller.bMap.Name + " [" + controller.bMap.Difficulty + "]")
	name = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "'", "<", "_", ">", "_", "|", "_").Replace(name)
	path := filepath.Join(dir, fmt.Sprintf("%s - %s.osr", name, now.Format("20060102-150405.000")))

	data, err := rplpa.WriteReplay(replay)
	if err != nil {
		controller.replaySaved = false
		return fmt.Errorf("serialize replay: %w", err)
	}
	if err = os.WriteFile(path, data, 0644); err != nil {
		controller.replaySaved = false
		return fmt.Errorf("write replay: %w", err)
	}

	log.Printf("Saved local replay: %s", path)
	return nil
}
