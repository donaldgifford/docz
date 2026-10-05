package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// pageProperty is the docz content property's value (DESIGN-0020 §3).
type pageProperty struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Hash    string `json:"hash"`
	Version int    `json:"version"`
	Docz    string `json:"docz"`
}

// reconcile renders one item and brings its page in line, following the
// §3 flowchart: create, skip, leave unchanged, or update. An error fails
// the page; the result carries what was known when it did.
func (r *exportRun) reconcile(ctx context.Context, it *item, parentID string) (PageResult, error) {
	res := PageResult{ID: it.docID, Title: it.title, Source: it.source}

	opts := r.plan.render
	opts.Source, opts.Title = it.source, it.override
	opts.Resolve = r.plan.resolver(r.opts.Resolve)
	opts.SourceURL = r.sourceURL(it.source)

	rendered, err := Render(it.src, opts)
	if err != nil {
		return res, err
	}

	res.Title, res.Links, res.Body = rendered.Title, rendered.Links, rendered.Body

	page, err := r.opts.Client.FindPage(ctx, r.spaceID, rendered.Title)
	if err != nil {
		return res, err
	}

	if page == nil {
		return r.create(ctx, it, &rendered, parentID, res)
	}

	res.PageID, res.URL, res.Version = page.ID, page.WebURL, page.Version

	prop, stored, err := r.property(ctx, page.ID)
	if err != nil {
		return res, err
	}

	switch {
	case prop == nil && !r.opts.Force:
		res.Action, res.Reason = Skipped, "not docz's page"

		return res, nil
	case prop != nil && stored.Version != page.Version && !r.opts.Force:
		res.Action = Skipped
		res.Reason = fmt.Sprintf("edited in Confluence (v%d, expected v%d)", page.Version, stored.Version)

		return res, nil
	case prop != nil && stored.Hash == rendered.Hash && page.ParentID == parentID:
		res.Action = Unchanged

		return res, nil
	}

	return r.update(ctx, it, &rendered, page, prop, parentID, res)
}

// sourceURL asks the caller's resolver where a page's own source is
// browsable, for the banner.
func (r *exportRun) sourceURL(source string) string {
	if source == "" || r.opts.Resolve == nil {
		return ""
	}

	return r.opts.Resolve(source, path.Base(source)).URL
}

// property reads a page's docz property. A property whose value does not
// decode is reported present with a zero value, so it reads as edited.
func (r *exportRun) property(ctx context.Context, pageID string) (*Property, pageProperty, error) {
	var stored pageProperty

	prop, err := r.opts.Client.Property(ctx, pageID, propertyKey)
	if err != nil || prop == nil {
		return nil, stored, err
	}

	if err := json.Unmarshal(prop.Value, &stored); err != nil {
		stored = pageProperty{Version: -1}
	}

	return prop, stored, nil
}

// create writes a new page and its property.
func (r *exportRun) create(
	ctx context.Context,
	it *item,
	rendered *Rendered,
	parentID string,
	res PageResult, //nolint:gocritic // a value in, a value out: the result is the caller's copy
) (PageResult, error) {
	res.Action = Created
	if r.opts.DryRun {
		return res, nil
	}

	page, err := r.opts.Client.CreatePage(ctx, &NewPage{
		SpaceID: r.spaceID, ParentID: parentID, Title: rendered.Title, Body: rendered.Body,
	})
	if err != nil {
		return res, err
	}

	res.PageID, res.URL, res.Version = page.ID, page.WebURL, page.Version

	return res, r.writeProperty(ctx, page.ID, nil, it, rendered.Hash, page.Version)
}

// update writes a new version of a page and its property.
func (r *exportRun) update(
	ctx context.Context,
	it *item,
	rendered *Rendered,
	page *Page,
	prop *Property,
	parentID string,
	res PageResult, //nolint:gocritic // a value in, a value out: the result is the caller's copy
) (PageResult, error) {
	res.Action = Updated
	if r.opts.DryRun {
		return res, nil
	}

	updated, err := r.opts.Client.UpdatePage(ctx, page.ID, &PageUpdate{
		Title:    rendered.Title,
		ParentID: parentID,
		Body:     rendered.Body,
		Version:  page.Version + 1,
		Message:  syncMessage(it, rendered.Hash),
	})
	if err != nil {
		return res, err
	}

	res.URL, res.Version = updated.WebURL, updated.Version

	return res, r.writeProperty(ctx, page.ID, prop, it, rendered.Hash, updated.Version)
}

// writeProperty creates the docz property, or updates prop through its own
// version.
func (r *exportRun) writeProperty(ctx context.Context, pageID string, prop *Property, it *item, hash string, version int) error {
	value, err := json.Marshal(pageProperty{
		ID: it.key, Source: it.source, Hash: hash, Version: version, Docz: r.opts.Version,
	})
	if err != nil {
		return err
	}

	next := &Property{Key: propertyKey, Value: value}
	if prop != nil {
		next.ID, next.Version = prop.ID, prop.Version
	}

	return r.opts.Client.SetProperty(ctx, pageID, next)
}

// syncMessage is the version comment: docz sync <id> <hash prefix>.
func syncMessage(it *item, hash string) string {
	digest := strings.TrimPrefix(hash, "sha256:")
	if len(digest) > 12 {
		digest = digest[:12]
	}

	return "docz sync " + it.key + " " + digest
}
