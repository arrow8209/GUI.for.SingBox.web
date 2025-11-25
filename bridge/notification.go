package bridge

import "log"

func (a *App) Notify(title string, message string, _ string, _ NotifyOptions) FlagResult {
	log.Printf("Notify: %s - %s", title, message)
	return FlagResult{true, "Success"}
}
