package main

import (
	"fmt"
	"strings"
)

// fuzzBudget is the time spent per target. Continuous fuzzing here means
// budgeted, not heroic (see the sec-fuzzing skill): a periodic job runs each
// target for this long, while plain `go test ./...` always replays the
// committed corpus for free.
const fuzzBudget = "30s"

// fuzz enumerates every Fuzz* target in the root module and runs each for
// fuzzBudget. Go's fuzzer runs one target at a time, so we discover them per
// package and loop. Any crasher fails the run; commit the saved testdata as a
// permanent regression seed. Like vulncheck, this is not part of `all`.
func fuzz(root string) error {
	pkgs, err := goListPackages(root)
	if err != nil {
		return err
	}
	total := 0
	for _, pkg := range pkgs {
		fns, err := fuzzTargets(root, pkg)
		if err != nil {
			return fmt.Errorf("fuzz list %s: %w", pkg, err)
		}
		for _, fn := range fns {
			fmt.Printf("fuzz %s %s (%s)\n", pkg, fn, fuzzBudget)
			if err := run(root, nil, "go", "test", pkg, "-run", "^$",
				"-fuzz", "^"+fn+"$", "-fuzztime", fuzzBudget); err != nil {
				return fmt.Errorf("fuzz %s %s: %w", pkg, fn, err)
			}
			total++
		}
	}
	fmt.Printf("fuzz: %d target(s) clean (%s each)\n", total, fuzzBudget)
	return nil
}

// goListPackages returns the import paths of the root module's packages.
func goListPackages(root string) ([]string, error) {
	out, err := runOut(root, "go", "list", "./...")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// fuzzTargets returns the Fuzz* function names defined in pkg, via
// `go test -list`. Packages without fuzz targets yield none.
func fuzzTargets(root, pkg string) ([]string, error) {
	out, err := runOut(root, "go", "test", "-list", "^Fuzz", pkg)
	if err != nil {
		return nil, err
	}
	var fns []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Fuzz") {
			fns = append(fns, line)
		}
	}
	return fns, nil
}
