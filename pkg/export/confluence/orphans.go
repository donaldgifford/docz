package confluence

import (
	"context"
	"encoding/json"
	"strings"
)

// orphans moves every docz page under the parent or a type page whose
// property id is no longer exported to the Archive child of the parent
// (DESIGN-0020 §3). Only a full run calls it: a narrowed run's set is
// partial, and everything outside it would read as an orphan.
func (r *exportRun) orphans(ctx context.Context) error {
	for _, containerID := range r.containers() {
		if err := ctx.Err(); err != nil {
			return err
		}

		children, err := r.opts.Client.Children(ctx, containerID)
		if err != nil {
			r.record(ctx, &PageResult{Title: "children of " + containerID}, err)

			continue
		}

		for i := range children {
			if err := ctx.Err(); err != nil {
				return err
			}

			r.orphan(ctx, &children[i])
		}
	}

	return nil
}

// containers are the page ids orphans are looked for under: the parent
// and every type page that exists.
func (r *exportRun) containers() []string {
	var out []string

	for i := range r.plan.items {
		it := &r.plan.items[i]
		if id := r.pageIDs[i]; id != "" && (it.parent < 0 || strings.HasPrefix(it.key, typeKeyPrefix)) {
			out = append(out, id)
		}
	}

	return out
}

// orphan archives one child page when it is docz's and no longer exported.
func (r *exportRun) orphan(ctx context.Context, child *Page) {
	if child.Title == archiveTitle {
		return
	}

	prop, stored, err := r.property(ctx, child.ID)
	if err != nil {
		r.record(ctx, &PageResult{Title: child.Title, PageID: child.ID}, err)

		return
	}

	if prop == nil || r.plan.keys[stored.ID] {
		return
	}

	res := PageResult{ID: stored.ID, Title: child.Title, Source: stored.Source, Action: Archived, PageID: child.ID}

	if !r.opts.DryRun {
		page, err := r.archive(ctx, child, prop, &stored)
		if page != nil {
			res.URL, res.Version = page.WebURL, page.Version
		}

		r.record(ctx, &res, err)

		return
	}

	r.record(ctx, &res, nil)
}

// archive moves a page under Archive, body untouched, and brings its
// property's version along so the page does not read as edited when its
// document returns.
func (r *exportRun) archive(ctx context.Context, child *Page, prop *Property, stored *pageProperty) (*Page, error) {
	archiveID, err := r.archivePage(ctx)
	if err != nil {
		return nil, err
	}

	moved, err := r.opts.Client.UpdatePage(ctx, child.ID, &PageUpdate{
		Title: child.Title, ParentID: archiveID, Message: "docz sync: archived, " + stored.ID + " is no longer exported",
	})
	if err != nil {
		return nil, err
	}

	stored.Version = moved.Version

	value, err := json.Marshal(stored)
	if err != nil {
		return moved, err
	}

	return moved, r.opts.Client.SetProperty(ctx, child.ID, &Property{
		ID: prop.ID, Key: propertyKey, Value: value, Version: prop.Version,
	})
}

// archivePage finds or creates the Archive page under the parent.
func (r *exportRun) archivePage(ctx context.Context) (string, error) {
	if r.archiveID != "" {
		return r.archiveID, nil
	}

	page, err := r.opts.Client.FindPage(ctx, r.spaceID, archiveTitle)
	if err != nil {
		return "", err
	}

	if page == nil {
		page, err = r.opts.Client.CreatePage(ctx, &NewPage{
			SpaceID:  r.spaceID,
			ParentID: r.pageIDs[0],
			Title:    archiveTitle,
			Body:     []byte("<p>Pages docz exported whose documents no longer exist. Nothing here is deleted.</p>"),
		})
		if err != nil {
			return "", err
		}
	}

	r.archiveID = page.ID

	return page.ID, nil
}
