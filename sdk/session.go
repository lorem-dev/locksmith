package sdk

func HideSession(sessionId string) string {
	if len(sessionId) < 15 {
		return "****"
	}
	if len(sessionId) < 30 {
		return sessionId[:5] + "****" + sessionId[len(sessionId)-4:]
	}
	return sessionId[:5] + "****" + sessionId[len(sessionId)-10:]
}
