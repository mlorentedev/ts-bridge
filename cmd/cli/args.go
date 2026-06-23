package cmd

// NormalizeArgs returns a copy of argv (the full process argument vector,
// including the program name at index 0) with legacy flag shorthands rewritten
// so Cobra parses them correctly. A standalone "-v" token is mapped to
// "--verbose". The input slice is never mutated.
//
// This replaces an in-main rewrite that derived the rewrite index from
// `range os.Args[1:]` while writing back into os.Args — an off-by-one that
// overwrote the argument *before* "-v" (e.g. the value of --target) with
// "--verbose", producing errors like "target invalid format: address
// --verbose: missing port in address" (issue #211). Iterating real indices on
// a fresh copy removes the hazard.
func NormalizeArgs(argv []string) []string {
	out := make([]string, len(argv))
	copy(out, argv)
	for i := 1; i < len(out); i++ {
		if out[i] == "-v" {
			out[i] = "--verbose"
		}
	}
	return out
}

// LegacyVersionRequested reports whether argv (the full process argument
// vector) contains a legacy "-version" or "--version" flag. Cobra exposes
// version information through the "version" subcommand rather than a
// --version flag, so these pre-Cobra forms are handled explicitly by the
// caller before command dispatch.
func LegacyVersionRequested(argv []string) bool {
	for i := 1; i < len(argv); i++ {
		if argv[i] == "-version" || argv[i] == "--version" {
			return true
		}
	}
	return false
}
