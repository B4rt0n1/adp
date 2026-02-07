package server

import "time"

const (
	cookieName       = "goedu_session"
	sessionTTL       = 7 * 24 * time.Hour
	loginRateWindow  = 1 * time.Minute
	loginRateMaxHits = 10
	bcryptCost       = 12
)
