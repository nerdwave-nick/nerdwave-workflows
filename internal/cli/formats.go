package cli

func validOutputFormat(format string) bool {
	return format == "cli" || format == "markdown" || format == "json"
}

// Normalize only persisted preferences. Public command flags deliberately reject
// the former human name. The stored client and its revision remain untouched.
func normalizeOutputFormat(format string) string {
	if format == "human" {
		return "cli"
	}
	return format
}
