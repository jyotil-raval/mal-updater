package config

const (
	// MAL OAuth2 endpoints
	MALAuthURL  = "https://myanimelist.net/v1/oauth2/authorize"
	MALTokenURL = "https://myanimelist.net/v1/oauth2/token"

	// MAL API
	MALAPIBaseURL        = "https://api.myanimelist.net/v2"
	MALAnimeListEndpoint = "/users/@me/animelist"

	// OAuth2 flow
	CallbackPort     = "8080"
	CallbackPath     = "/callback"
	PKCEMethod       = "plain"
	GrantTypeAuth    = "authorization_code"
	GrantTypeRefresh = "refresh_token"

	// Token storage
	TokenFile         = "token.json"
	TokenExpireBuffer = 5 // minutes before expiry to trigger refresh

	// API limits
	MALListLimit = 1000 // max entries per page MAL allows
)
