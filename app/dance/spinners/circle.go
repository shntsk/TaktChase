package spinners

import (
	"github.com/shntsk/taktchase/app/settings"
	"github.com/shntsk/taktchase/framework/math/math32"
	"github.com/shntsk/taktchase/framework/math/vector"
)

type CircleMover struct {
	*BaseMover
}

func NewCircleMover() *CircleMover {
	return &CircleMover{BaseMover: &BaseMover{}}
}

func (c *CircleMover) GetPositionAt(time float64) vector.Vector2f {
	spS := settings.CursorDance.Spinners[c.id%len(settings.CursorDance.Spinners)]
	return vector.NewVec2fRad(rpms*c.GetSDelta(time)*2*math32.Pi, float32(spS.Radius)).Add(center.AddS(float32(spS.CenterOffsetX), float32(spS.CenterOffsetY)))
}
