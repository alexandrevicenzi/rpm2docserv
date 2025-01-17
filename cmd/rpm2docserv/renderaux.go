package main

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thkukuk/rpm2docserv/pkg/bundled"
	"github.com/thkukuk/rpm2docserv/pkg/manpage"
	"github.com/thkukuk/rpm2docserv/pkg/write"
)

var indexTmpl = mustParseIndexTmpl()

func mustParseIndexTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("index").Parse(bundled.Asset("index.tmpl")))
}

var aboutTmpl = mustParseAboutTmpl()

func mustParseAboutTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("about").Parse(bundled.Asset("about.tmpl")))
}

type bySuiteStr []string

func (p bySuiteStr) Len() int      { return len(p) }
func (p bySuiteStr) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p bySuiteStr) Less(i, j int) bool {
	orderi, oki := sortOrder[p[i]]
	orderj, okj := sortOrder[p[j]]
	if !oki || !okj {
		panic(fmt.Sprintf("either %q or %q is an unknown suite. known: %+v", p[i], p[j], sortOrder))
	}
	return orderi < orderj
}


func renderAux(destDir string, gv globalView) error {
	suites := make([]string, 0, len(gv.suites))
	for suite := range gv.suites {
		suites = append(suites, suite)
	}
	sort.Stable(bySuiteStr(suites))

	if err := write.Atomically(filepath.Join(destDir, "index.html.gz"), true, func(w io.Writer) error {
		return indexTmpl.Execute(w, struct {
			Title          string
			Rpm2docservVersion string
			Breadcrumbs    breadcrumbs
			FooterExtra    string
			Suites         []string
			Meta           *manpage.Meta
			HrefLangs      []*manpage.Meta
		}{
			Title:          "index",
			Suites:         []string{"manpages"},
			Rpm2docservVersion: rpm2docservVersion,
		})
	}); err != nil {
		return err
	}

	if err := write.Atomically(filepath.Join(destDir, "about.html.gz"), true, func(w io.Writer) error {
		return aboutTmpl.Execute(w, struct {
			Title          string
			Rpm2docservVersion string
			Breadcrumbs    breadcrumbs
			FooterExtra    string
			Meta           *manpage.Meta
			HrefLangs      []*manpage.Meta
		}{
			Title:          "About",
			Rpm2docservVersion: rpm2docservVersion,
		})
	}); err != nil {
		return err
	}

	for name, content := range bundled.AssetsFiltered(func(fn string) bool {
		return !strings.HasSuffix(fn, ".tmpl") && !strings.HasSuffix(fn, "style.css")
	}) {
		if err := write.Atomically(filepath.Join(destDir, filepath.Base(name)), false, func(w io.Writer) error {
			_, err := io.WriteString(w, content)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}
