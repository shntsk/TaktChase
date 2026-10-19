package schedulers

import (
	"github.com/shntsk/taktchase/app/beatmap/difficulty"
	"github.com/shntsk/taktchase/app/beatmap/objects"
	"github.com/shntsk/taktchase/app/dance/spinners"
	"github.com/shntsk/taktchase/app/graphics"
)

type Scheduler interface {
	Init(objects []objects.IHitObject, diff *difficulty.Difficulty, cursor *graphics.Cursor, spinnerMoverCtor func() spinners.SpinnerMover, initKeys bool)
	Update(time float64)
}
