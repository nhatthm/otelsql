package otelsql

// Version is the current release version of the otelsql instrumentation.
func Version() string {
	// This string is updated by the pre_release.sh script during release
	return "0.1.1"
}

// SemVersion is the semantic version to be supplied to tracer/meter creation.
func SemVersion() string {
	return "semver:" + Version()
}
