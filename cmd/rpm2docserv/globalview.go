package main

import (
	// "compress/gzip"
	// "encoding/json"
	"fmt"
	// "io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thkukuk/rpm2docserv/pkg/manpage"
	"github.com/thkukuk/rpm2docserv/pkg/rpm"
)

type pkgEntry struct {
        source    string
	suite     string
        binarypkg string
        arch      string
        filename  string
        version   string
}

type stats struct {
	PackagesExtracted uint64
	ManpagesRendered  uint64
	ManpageBytes      uint64
	HTMLBytes         uint64
	IndexBytes        uint64
}

type globalView struct {
	// pkgs contains all binary packages we know of.
	pkgs []*pkgEntry

        // suites is always "manpages", but leave it if needed later
	// and to make things easier.
        suites map[string]bool

        // idxSuites maps codename, suite and command-line argument to suite (as in
        // suites).
        // e.g. map[oldoldstable:wheezy wheezy:wheezy]
        idxSuites map[string]string

	// xref maps from manpage.Meta.Name (e.g. “w3m” or “systemd.service”) to
	// the corresponding manpage.Meta.
	xref map[string][]*manpage.Meta

	stats *stats
	start time.Time
}

func markPresent(latestVersion map[string]*manpage.PkgMeta, xref map[string][]*manpage.Meta, filename string, key string) error {
        if _, ok := latestVersion[key]; !ok {
                return fmt.Errorf("Could not determine latest version")
        }
        m, err := manpage.FromManPath(strings.TrimPrefix(filename, "usr/share/man/"), latestVersion[key])
        if err != nil {
                return fmt.Errorf("Trying to interpret path %q: %v", filename, err)
        }
        // NOTE(stapelberg): this additional verification step
        // is necessary because manpages such as the French
        // manpage for qelectrotech(1) are present in multiple
        // encodings. manpageFromManPath ignores encodings, so
        // if we didn’t filter, we would end up with what
        // looks like duplicates.
        present := false
        for _, x := range xref[m.Name] {
                if x.ServingPath() == m.ServingPath() {
                        present = true
                        break
                }
        }
        if !present {
                xref[m.Name] = append(xref[m.Name], m)
        }
        return nil
}

// go through the cache directory, find all RPMs and build a pkg entry for it
func buildGlobalView(cacheDir string, start time.Time) (globalView, error) {
	var stats stats
	res := globalView{
		suites:        make(map[string]bool, 1),
		xref:          make(map[string][]*manpage.Meta),
		stats:         &stats,
		start:         start,
	}

	// we currently have only a "dummy" suite, manpages
	suite := "manpages"
	res.suites[suite] = true
	// and a dummy component
	component := "unknown"

	latestVersion := make(map[string]*manpage.PkgMeta)

	// Walk recursivly through the full cache directory, search all
	// RPMs and store the meta data for them.
	err := filepath.Walk(cacheDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".rpm") {
				// Add RPM to package list
				pkg := new(pkgEntry)
				// We don't have "suites" yet
				pkg.suite = suite
				pkg.filename = path

				var version, release string
				pkg.binarypkg, version, release, pkg.arch, err = rpm.SplitRPMname(filepath.Base(path))
				if err != nil {
					return err
				}
				pkg.version = version + "-" + release;

				pkg.source, err = rpm.GetSourceRPMName(path)
				if err != nil {
					return err
				}
				// We don't need the version and rest of the source RPM name
				pkg.source, _, _, _, err = rpm.SplitRPMname(pkg.source)

				res.pkgs = append (res.pkgs, pkg)

				latestVersion[suite + "/" + pkg.binarypkg] = &manpage.PkgMeta{
					Component: component,
					Filename: path,
					Sourcepkg: pkg.source,
					Binarypkg: pkg.binarypkg,
					Version: pkg.version,
					Suite: suite,
				}
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}

	content, err := getAllContents(suite, res.pkgs)
	if err != nil {
		return res, err
	}

	knownIssues := make(map[string][]error)

	// Build a global view of all the manpages (required for cross-referencing).
	for _, c := range content {
		key := c.suite + "/" + c.binarypkg
		if err := markPresent(latestVersion, res.xref, c.filename, key); err != nil {
			knownIssues[key] = append(knownIssues[key], err)
		}
	}

	for key, errors := range knownIssues {
		log.Printf("package %q has errors: %v", key, errors)
	}

	return res, err
}