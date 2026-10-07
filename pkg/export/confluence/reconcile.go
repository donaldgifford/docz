package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// pageProperty is the docz content property's value (DESIGN-0020 §3,
// DESIGN-0021 §2).
type pageProperty struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Hash    string `json:"hash"`
	Version int    `json:"version"`
	Docz    string `json:"docz"`
	// Repo is the owner/name that wrote the page. Phase A wrote none, and
	// a property without one belongs to any repository.
	Repo string `json:"repo,omitempty"`
}

// owned reports whether the repository named repo may write the page:
// the property names no repository or names this one.
func (p *pageProperty) owned(repo string) bool {
	return p.Repo == "" || p.Repo == repo
}

// valid reports whether a property is one docz export wrote: it names a
// document and a page version. Another tool's property under the same key,
// or one that does not decode, is not, and the page it is on is treated as
// somebody else's: skipped without --force and never archived.
func (p *pageProperty) valid() bool {
	return p.ID != "" && p.Version > 0
}

// reconcile renders one item and brings its page in line, following the
// §3 flowchart: create, skip, leave unchanged, or update. An error fails
// the page; the result carries what was known when it did.
func (r *exportRun) reconcile(ctx context.Context, it *item, parentID string) (PageResult, error) {
	res := PageResult{ID: it.docID, Key: it.key, Title: it.title, Source: it.source}

	opts := r.plan.render
	opts.Source, opts.Title = it.source, it.override
	opts.Resolve = r.plan.resolver(r.opts.Resolve)
	opts.SourceURL = r.sourceURL(it.source)

	rendered, err := Render(it.src, opts)
	if err != nil {
		return res, err
	}

	res.Title, res.Links, res.Body, res.Hash = rendered.Title, rendered.Links, rendered.Body, rendered.Hash

	page, err := r.findPage(ctx, it.key, rendered.Title)
	if err != nil {
		return res, err
	}

	if page == nil {
		return r.create(ctx, it, &rendered, parentID, res)
	}

	res.PageID, res.URL, res.Version = page.ID, page.WebURL, page.Version

	// In the page layout the parent page stays wherever it is: docz places
	// what is beneath it, not the page itself, and Confluence files a page
	// created with no parent under the space's homepage rather than at the
	// root. In the folder layout the home page belongs in the folder.
	if it.key == parentKey && r.plan.folder == "" {
		parentID = page.ParentID
	}

	prop, stored, err := r.property(ctx, PageTarget(page.ID))
	if err != nil {
		return res, err
	}

	if prop != nil && !stored.owned(r.opts.Repository) {
		res.Action, res.Reason = Skipped, "belongs to "+stored.Repo

		return res, nil
	}

	d := decide(prop, &stored, page, &want{hash: rendered.Hash, parentID: parentID, title: rendered.Title}, r.opts)
	res.Edited = d.edited

	if d.action != Updated {
		res.Action, res.Reason = d.action, d.reason

		return res, nil
	}

	return r.update(ctx, it, &rendered, page, prop, parentID, res)
}

// findPage finds an item's page: by the id the caller recorded for its key
// when that page is still in the space, else by title.
func (r *exportRun) findPage(ctx context.Context, key, title string) (*Page, error) {
	if id := r.opts.Pages[key]; id != "" {
		page, err := r.opts.Client.Page(ctx, id)
		if err != nil {
			return nil, err
		}

		if page != nil && page.SpaceID == r.spaceID {
			return page, nil
		}
	}

	return r.opts.Client.FindPage(ctx, r.spaceID, title)
}

// want is what a page should be after the run.
type want struct {
	hash     string
	parentID string
	title    string
}

// decision is decide's answer.
type decision struct {
	action Action
	reason string
	edited *Edit
}

// decide is the §3 flowchart for a page that exists and is not another
// repository's: Skipped with a reason, Unchanged, or Updated, which the
// caller then writes. A version that moved since docz wrote it is reported
// in edited whatever is done about it.
func decide(prop *Property, stored *pageProperty, page *Page, w *want, opts *ExportOptions) decision {
	ours := prop != nil && stored.valid()
	edited := editOf(ours, stored, page)
	same := ours && stored.Hash == w.hash && page.ParentID == w.parentID && page.Title == w.title

	switch {
	case opts.Force && same && edited == nil:
		return decision{action: Unchanged}
	case opts.Force:
		return decision{action: Updated, edited: edited}
	case prop == nil:
		return decision{action: Skipped, reason: "not docz's page"}
	case !ours:
		return decision{action: Skipped, reason: "its docz property was not written by docz export"}
	case edited != nil && opts.Overwrite:
		return decision{action: Updated, edited: edited}
	case edited != nil:
		return decision{
			action: Skipped,
			reason: fmt.Sprintf("edited in Confluence (v%d, expected v%d)", page.Version, stored.Version),
			edited: edited,
		}
	case same:
		return decision{action: Unchanged}
	default:
		return decision{action: Updated}
	}
}

// editOf reports a docz page whose version moved since docz wrote it.
func editOf(ours bool, stored *pageProperty, page *Page) *Edit {
	if !ours || stored.Version == page.Version {
		return nil
	}

	return &Edit{Version: page.Version, Expected: stored.Version}
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
// decode is reported present with a zero value, which valid rejects.
func (r *exportRun) property(ctx context.Context, t Target) (*Property, pageProperty, error) {
	var stored pageProperty

	prop, err := r.opts.Client.Property(ctx, t, propertyKey)
	if err != nil || prop == nil {
		return nil, stored, err
	}

	if err := json.Unmarshal(prop.Value, &stored); err != nil {
		stored = pageProperty{}
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
		ID: it.key, Source: it.source, Hash: hash, Version: version, Docz: r.opts.Version, Repo: r.opts.Repository,
	})
	if err != nil {
		return err
	}

	next := &Property{Key: propertyKey, Value: value}
	if prop != nil {
		next.ID, next.Version = prop.ID, prop.Version
	}

	return r.opts.Client.SetProperty(ctx, PageTarget(pageID), next)
}

// syncMessage is the version comment: docz sync <id> <hash prefix>.
func syncMessage(it *item, hash string) string {
	digest := strings.TrimPrefix(hash, "sha256:")
	if len(digest) > 12 {
		digest = digest[:12]
	}

	return "docz sync " + it.key + " " + digest
}
