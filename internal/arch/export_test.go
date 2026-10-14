package arch

// AllowedImportForTest re-exports the unexported allowedImport for the
// external test package. This keeps the matrix itself unexported (so it is not
// part of any API) while still being assertable from the black-box test.
func AllowedImportForTest(from, to Layer) bool { return allowedImport(from, to) }
