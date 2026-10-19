package overlays

import (
	"github.com/shntsk/taktchase/app/graphics"
	"github.com/shntsk/taktchase/framework/bass"
	"github.com/shntsk/taktchase/framework/graphics/batch"
	color2 "github.com/shntsk/taktchase/framework/math/color"
)

type Overlay interface {
	Update(float64)
	SetMusic(bass.ITrack)
	DrawBackground(batch *batch.QuadBatch, colors []color2.Color, alpha float64)
	DrawBeforeObjects(batch *batch.QuadBatch, colors []color2.Color, alpha float64)
	DrawNormal(batch *batch.QuadBatch, colors []color2.Color, alpha float64)
	DrawHUD(batch *batch.QuadBatch, colors []color2.Color, alpha float64)
	IsBroken(cursor *graphics.Cursor) bool
	DisableAudioSubmission(b bool)
	ShouldDrawHUDBeforeCursor() bool
}
