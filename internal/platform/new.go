package platform

// New assembles a Platform using the operating-system specific providers. The
// concrete constructors (newPortResolver, newNetworkInspector, newClipboardProvider)
// are defined in build-tagged files so the correct implementation is compiled
// for each target OS.
func New() *Platform {
	return &Platform{
		Ports:      newPortResolver(),
		Processes:  newProcessInspector(),
		Network:    newNetworkInspector(),
		Tree:       newProcessTreeProvider(),
		Clipboard:  newClipboardProvider(),
		Controller: newProcessController(),
		Containers: newContainerProvider(),
	}
}
