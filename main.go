// Command regexpl is a regex test-and-explain service.
//
// It parses RE2 patterns into a human-readable syntax tree (explain) and runs
// them against input text (match), reporting match spans and capture groups.
// Invalid patterns are reported with a classified error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"regexpl/internal/regexpl"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: regexpl <explain|match|--smoke-test> [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "explain":
		runExplain(os.Args[2:])
	case "match":
		runMatch(os.Args[2:])
	case "--smoke-test":
		runSmokeTest()
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  regexpl explain --pattern PATTERN")
	fmt.Fprintln(os.Stderr, "  regexpl match --pattern PATTERN --input TEXT [--global]")
	fmt.Fprintln(os.Stderr, "  regexpl --smoke-test")
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	pattern := fs.String("pattern", "", "regex pattern to explain")
	fs.Parse(args)
	if *pattern == "" {
		fmt.Fprintln(os.Stderr, "explain: --pattern is required")
		os.Exit(2)
	}
	emit(regexpl.Explain(*pattern))
}

func runMatch(args []string) {
	fs := flag.NewFlagSet("match", flag.ExitOnError)
	pattern := fs.String("pattern", "", "regex pattern to match")
	input := fs.String("input", "", "input text to match against")
	global := fs.Bool("global", false, "return all non-overlapping matches")
	fs.Parse(args)
	if *pattern == "" {
		fmt.Fprintln(os.Stderr, "match: --pattern is required")
		os.Exit(2)
	}
	var res regexpl.MatchResult
	if *global {
		res = regexpl.MatchAll(*pattern, *input)
	} else {
		res = regexpl.MatchFirst(*pattern, *input)
	}
	emit(res)
}

func runSmokeTest() {
	if err := regexpl.SelfCheck(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("SMOKE-OK")
}

func emit(v any) {
	// HTML escaping is disabled so that '<' and '>' in regex group syntax such
	// as (?P<name>) are emitted verbatim instead of </>.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "internal error:", err)
		os.Exit(1)
	}
	out := bytes.TrimRight(buf.Bytes(), "\n")
	os.Stdout.Write(out)
	os.Stdout.Write([]byte("\n"))
}
