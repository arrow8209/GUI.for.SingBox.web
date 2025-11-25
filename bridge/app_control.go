package bridge

// ExitApp requests the hosting process to shut down.
func (a *App) ExitApp() {
	if a.Exit != nil {
		a.Exit()
	}
}
