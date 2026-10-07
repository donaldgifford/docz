package confluence

import (
	"context"
	"encoding/json"
	"strings"
)

// orphans moves every docz page in the folder (the parent page in the
// page layout) or a type page whose property id is no longer exported to
// the Archive page (DESIGN-0020 §3, DESIGN-0021 §3). Only a full run calls
// it: a narrowed run's set is partial, and everything outside it would
// read as an orphan.
func (r *exportRun) orphans(ctx context.Context) error {
	for _, container := range r.containers() {
		if err := ctx.Err(); err != nil {
			return err
		}

		children, err := r.opts.Client.Children(ctx, container)
		if err != nil {
			r.record(ctx, &PageResult{Title: "children of " + container.ID}, err)

			continue
		}

		for i := range children {
			if err := ctx.Err(); err != nil {
				return err
			}

			// A folder is never a candidate: docz moves only pages.
			if children[i].Type == TypePage {
				r.orphan(ctx, &children[i].Page)
			}
		}
	}

	return nil
}

// containers are where orphans are looked for: the folder, or the parent
// page in the page layout, and every type page that exists.
func (r *exportRun) containers() []Target {
	var out []Target

	if r.folderID != "" {
		out = append(out, FolderTarget(r.folderID))
	}

	for i := range r.plan.items {
		it := &r.plan.items[i]

		id := r.pageIDs[i]
		if id == "" {
			continue
		}

		if strings.HasPrefix(it.key, typeKeyPrefix) || (it.key == parentKey && r.plan.folder == "") {
			out = append(out, PageTarget(id))
		}
	}

	return out
}

// orphan archives one child page when it is this repository's and no
// longer exported.
func (r *exportRun) orphan(ctx context.Context, child *Page) {
	// A page this run writes, or Archive itself, is never an orphan, and
	// knowing that by title saves a property request per page.
	if child.Title == r.plan.archive || r.plan.titles[child.Title] {
		return
	}

	prop, stored, err := r.property(ctx, PageTarget(child.ID))
	if err != nil {
		r.record(ctx, &PageResult{Title: child.Title, PageID: child.ID}, err)

		return
	}

	if prop == nil || !stored.valid() || !stored.owned(r.opts.Repository) || r.plan.keys[stored.ID] {
		return
	}

	res := PageResult{ID: stored.ID, Key: stored.ID, Title: child.Title, Source: stored.Source, Action: Archived, PageID: child.ID}

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

	return moved, r.opts.Client.SetProperty(ctx, PageTarget(child.ID), &Property{
		ID: prop.ID, Key: propertyKey, Value: value, Version: prop.Version,
	})
}

// archivePage finds or creates the Archive page: in the folder, or under
// the parent page in the page layout.
func (r *exportRun) archivePage(ctx context.Context) (string, error) {
	if r.archiveID != "" {
		return r.archiveID, nil
	}

	page, err := r.opts.Client.FindPage(ctx, r.spaceID, r.plan.archive)
	if err != nil {
		return "", err
	}

	if page == nil {
		parentID := r.folderID
		if parentID == "" {
			parentID = r.pageIDs[0]
		}

		page, err = r.opts.Client.CreatePage(ctx, &NewPage{
			SpaceID:  r.spaceID,
			ParentID: parentID,
			Title:    r.plan.archive,
			Body:     []byte("<p>Pages docz exported whose documents no longer exist. Nothing here is deleted.</p>"),
		})
		if err != nil {
			return "", err
		}
	}

	r.archiveID = page.ID

	return page.ID, nil
}
