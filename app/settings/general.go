package settings

import (
	"log"
	"os"
	"path/filepath"

	"github.com/shntsk/taktchase/framework/env"
)

// ensureDir creates the directory if it doesn't exist yet.
func ensureDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Println("Settings: Failed to create directory", dir+":", err)
	}
}

var General = initGeneral()

func initGeneral() *general {
	// Folders live next to the game (relative paths are resolved against the data dir)
	// and are created automatically, no osu! installation is looked up.
	return &general{
		OsuSongsDir:       "Songs",
		OsuSkinsDir:       "Skins",
		OsuReplaysDir:     "Replays",
		DiscordPresenceOn: true,
		UnpackOszFiles:    true,
		VerboseImportLogs: false,
	}
}

type general struct {
	// Directory that contains osu! songs
	OsuSongsDir string `long:"true" label:"osu! Songs directory" path:"Select osu! Songs directory"`

	// Directory that contains osu! skins
	OsuSkinsDir string `long:"true" label:"osu! Skins directory" path:"Select osu! Skins directory"`

	// Directory that contains osu! replays
	OsuReplaysDir string `long:"true" label:"osu! Replays directory" path:"Select osu! Replays directory" tooltip:"Don't use replays directory inside taktchase's directory!"`

	// Whether discord should show that taktchase is on
	DiscordPresenceOn bool `label:"Discord Rich Presence"`

	// Whether taktchase should unpack .osz files in Songs folder, osu! may complain about it
	UnpackOszFiles bool

	// Whether import details should be shown. If false, only failures will be logged.
	VerboseImportLogs bool

	songsDir   *string
	skinsDir   *string
	replaysDir *string
}

func (g *general) GetSongsDir() string {
	if g.songsDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuSongsDir)

		if filepath.IsAbs(g.OsuSongsDir) {
			dir = g.OsuSongsDir
		}

		ensureDir(dir)

		g.songsDir = &dir
	}

	return *g.songsDir
}

func (g *general) GetSkinsDir() string {
	if g.skinsDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuSkinsDir)

		if filepath.IsAbs(g.OsuSkinsDir) {
			dir = g.OsuSkinsDir
		}

		ensureDir(dir)

		g.skinsDir = &dir
	}

	return *g.skinsDir
}

func (g *general) GetReplaysDir() string {
	if g.replaysDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuReplaysDir)

		if filepath.IsAbs(g.OsuReplaysDir) {
			dir = g.OsuReplaysDir
		}

		ensureDir(dir)

		g.replaysDir = &dir
	}

	return *g.replaysDir
}
