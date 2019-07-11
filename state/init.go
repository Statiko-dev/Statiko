package state

// Instance is a singleton for Manager
var Instance *Manager

// Init the singleton
func init() {
	// Initialize the singleton
	Instance = &Manager{}
	if err := Instance.Init(); err != nil {
		panic(err)
	}
}
