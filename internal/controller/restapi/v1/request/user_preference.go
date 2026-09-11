package request

type SetPreferencesRequest struct {
	Categories []string `json:"categories" validate:"required,min=1,max=6,dive,oneof=Gaming Music Science Travel Film Lifestyle"`
}
