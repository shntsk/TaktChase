package settings

var Input = initInput()

func initInput() *input {
	return &input{
		LeftKey:              "Z",
		RightKey:             "X",
		RestartKey:           "`",
		SmokeKey:             "C",
		ScreenshotKey:        "F2",
		MouseButtonsDisabled: true,
		MouseHighPrecision:   false,
		MouseSensitivity:     1,
		AimAssist:            true,
		AimAssistStrength:    0.3,
	}
}

type input struct {
	LeftKey              string  `key:"true"`
	RightKey             string  `key:"true"`
	RestartKey           string  `key:"true"`
	SmokeKey             string  `key:"true"`
	ScreenshotKey        string  `key:"true"`
	MouseButtonsDisabled bool    `label:"Disable mouse buttons"`
	MouseHighPrecision   bool    `label:"Mouse raw input"`
	MouseSensitivity     float64 `label:"Raw input sensitivity" min:"0.4" max:"6"`
	AimAssist            bool    `label:"Aim assist"`
	AimAssistStrength    float64 `label:"Aim assist strength" min:"0.05" max:"1"`
}
