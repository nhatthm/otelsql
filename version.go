package otelsql

// Version is the current release version of the otelsql instrumentation.
func Version() string {
	return "0.2.1"
}

// SemVersion is the semantic version to be supplied to tracer/meter creation.
func SemVersion() string {
	return "semver:" + Version()
}
