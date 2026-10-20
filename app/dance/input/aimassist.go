package input

import (
	"github.com/shntsk/taktchase/app/graphics"
	"github.com/shntsk/taktchase/app/rulesets/osu"
	"github.com/shntsk/taktchase/framework/math/vector"
)

const (
	// How long before an object's start time the assist begins to consider it.
	assistLeadTime = 400.0
	// How long after an object's end time it is still considered (matches the widest hit window).
	assistLagTime = 150.0
)

// AimAssistProcessor nudges ONLY the position used for hit detection (cursor.RawPosition)
// toward the nearest active object. The visible cursor (cursor.Position) is never touched,
// so the assist is invisible to the player: it effectively makes hit areas and slider follow
// areas a bit more forgiving, never pulling the cursor from afar.
type AimAssistProcessor struct {
	cursor   *graphics.Cursor
	ruleset  *osu.OsuRuleSet
	strength func() float64
}

// NewAimAssistProcessor creates the processor. strength returns the current assist strength in [0, 1];
// it is read every frame so changes in the settings apply immediately.
func NewAimAssistProcessor(ruleset *osu.OsuRuleSet, cursor *graphics.Cursor, strength func() float64) *AimAssistProcessor {
	return &AimAssistProcessor{cursor: cursor, ruleset: ruleset, strength: strength}
}

// Update must be called after the cursor position was set from the real input and before the ruleset is updated.
func (processor *AimAssistProcessor) Update(time float64) {
	strength := processor.strength()
	if strength <= 0 {
		return
	}

	if strength > 1 {
		strength = 1
	}

	player := processor.ruleset.GetPlayer(processor.cursor)
	diff := processor.ruleset.GetPlayerDifficulty(processor.cursor)

	radius := float32(diff.GetRadius())
	maxPull := radius * float32(strength) // the hit area grows by at most this much
	realPos := processor.cursor.RawPosition

	var (
		bestTarget vector.Vector2f
		bestDist   float32
		found      bool
	)

	for _, o := range processor.ruleset.GetProcessed() {
		if _, isSpinner := o.(*osu.Spinner); isSpinner {
			continue
		}

		obj := o.GetObject()

		if time < obj.GetStartTime()-assistLeadTime || time > obj.GetEndTime()+assistLagTime {
			continue
		}

		if o.IsHit(player) {
			continue
		}

		target := obj.GetStackedPositionAtMod(time, diff)
		dist := realPos.Dst(target)

		if !found || dist < bestDist {
			bestTarget, bestDist, found = target, dist, true
		}
	}

	// Already inside the object, or too far away for the assist to matter.
	if !found || bestDist <= radius || bestDist > radius+maxPull {
		return
	}

	// Move just enough to land on the edge of the hit area (plus a tiny margin against rounding).
	pull := bestDist - radius + 0.01
	dir := bestTarget.Sub(realPos).Nor()

	processor.cursor.RawPosition = realPos.Add(dir.Scl(pull))
}
