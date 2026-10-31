package html

import (
	"encoding/base64"
	"io"
	"net/url"
	"path"
	"strings"
)

// maxInlineImage is the largest image Export writes into a page as a data
// URL; a larger one goes through the caller's resolver.
const maxInlineImage = 1 << 20

// imageTypes maps an image's extension to its MIME type.
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
}

// resolve places a relative link or image for a page of this run: a
// document the run exports becomes a relative link to its page, keeping the
// fragment; an image under the root of at most 1 MB becomes a data URL;
// anything else goes to ExportOptions.Resolve. A path that leaves the root
// is never read.
func (x *exporter) resolve(from, href string) string {
	target, frag, _ := strings.Cut(href, "#")
	target, _, _ = strings.Cut(target, "?")

	if unescaped, err := url.PathUnescape(target); err == nil {
		target = unescaped
	}

	if u := x.local(from, target, frag); u != "" {
		return u
	}

	if x.opts.Resolve == nil {
		return ""
	}

	return x.opts.Resolve(from, href)
}

// local places a target inside the repository: a page this run writes, or
// an image it inlines. "" means neither, including a target that leaves the
// root.
func (x *exporter) local(from, target, frag string) string {
	if target == "" || strings.HasPrefix(target, "/") {
		return ""
	}

	rel := path.Clean(path.Join(path.Dir(from), target))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}

	if page, ok := x.pages[rel]; ok {
		return relativeTo(x.pages[from], page) + fragment(frag)
	}

	return x.inline(rel)
}

// inline is rel as a data URL when it is an image under the root of at
// most maxInlineImage bytes, else "".
func (x *exporter) inline(rel string) string {
	mime, ok := imageTypes[strings.ToLower(path.Ext(rel))]
	if !ok {
		return ""
	}

	f, err := x.src.Open(rel)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read only.

	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() || st.Size() > maxInlineImage {
		return ""
	}

	b, err := io.ReadAll(io.LimitReader(f, maxInlineImage+1))
	if err != nil || len(b) > maxInlineImage {
		return ""
	}

	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// relativeTo is the path from the page at from to the page at to, both
// relative to the output directory.
func relativeTo(from, to string) string {
	fromDir := strings.Split(path.Dir(from), "/")
	if fromDir[0] == "." {
		fromDir = nil
	}

	toParts := strings.Split(to, "/")

	common := 0
	for common < len(fromDir) && common < len(toParts)-1 && fromDir[common] == toParts[common] {
		common++
	}

	parts := make([]string, 0, len(fromDir)-common+len(toParts)-common)
	for range fromDir[common:] {
		parts = append(parts, "..")
	}

	return strings.Join(append(parts, toParts[common:]...), "/")
}

// fragment is "#frag", or "" for none.
func fragment(frag string) string {
	if frag == "" {
		return ""
	}

	return "#" + frag
}
