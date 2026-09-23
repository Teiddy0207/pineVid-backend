package response

// Token -.
type Token struct {
	Token        string `json:"token"                   example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string `json:"refresh_token,omitempty" example:"q7Zb3k..."`
	// ExpiresIn is the access token lifetime in seconds.
	ExpiresIn int64 `json:"expires_in,omitempty" example:"3600"`
} // @name v1.Token
