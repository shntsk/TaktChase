package osu

import "github.com/shntsk/taktchase/app/settings"

const silentAimAssistRadiusMultiplier float32 = 9999

// getHitRadius implements Silent aim assist without moving the visible or raw cursor.
// Manual clicks and the normal timing windows are still required.
func (player *difficultyPlayer) getHitRadius() float32 {
	radius := player.diff.GetRadius()

	if settings.Input.AimAssist && player.cursor.IsPlayer && !player.cursor.IsAutoplay {
		return radius * silentAimAssistRadiusMultiplier
	}

	return radius
}
