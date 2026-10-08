package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// folderProperty is the docz content property on a repository's folder.
type folderProperty struct {
	Repo string `json:"repo"`
	Docz string `json:"docz"`
}

// resolveFolder finds the repository's folder, or creates it, and checks
// that it is this repository's (DESIGN-0021 §2). The recorded id comes
// first; then a folder with the title among the direct children of the
// parent page, or of the space's homepage when there is no parent. A
// folder another repository owns, or none does, stops the run.
func (r *exportRun) resolveFolder(ctx context.Context) error {
	folder, err := r.findFolder(ctx)
	if err != nil {
		return err
	}

	if folder == nil {
		return r.createFolder(ctx)
	}

	prop, err := r.opts.Client.Property(ctx, FolderTarget(folder.ID), propertyKey)
	if err != nil {
		return err
	}

	var stored folderProperty
	if prop != nil && json.Unmarshal(prop.Value, &stored) != nil {
		prop = nil
	}

	if prop == nil || stored.Repo != r.opts.Repository {
		owner := "no docz export"
		if prop != nil && stored.Repo != "" {
			owner = stored.Repo
		}

		return &ConfigError{Reason: fmt.Sprintf(
			"folder %q in space %s belongs to %s; set sync.confluence.folder to a title of this repository's own",
			folder.Title, r.syncSpace(), owner)}
	}

	r.useFolder(folder)

	return nil
}

// findFolder returns the recorded folder when it is still in the space,
// else the folder with the plan's title under the parent page or the
// space's homepage, else nil.
func (r *exportRun) findFolder(ctx context.Context) (*Folder, error) {
	if r.opts.Folder != "" {
		f, err := r.opts.Client.Folder(ctx, r.opts.Folder)
		if err != nil {
			return nil, err
		}

		if f != nil && f.SpaceID == r.spaceID {
			return f, nil
		}
	}

	containerID, err := r.folderParent(ctx)
	if err != nil || containerID == "" {
		return nil, err
	}

	children, err := r.opts.Client.Children(ctx, PageTarget(containerID))
	if err != nil {
		return nil, err
	}

	for i := range children {
		if c := &children[i]; c.Type == TypeFolder && c.Title == r.plan.folder {
			return &Folder{ID: c.ID, Title: c.Title, ParentID: c.ParentID, SpaceID: r.spaceID, WebURL: c.WebURL}, nil
		}
	}

	return nil, nil //nolint:nilnil // no folder yet is an answer: the caller creates it
}

// folderParent is the page the folder sits under: the parent page when
// sync.confluence.parent names one, which must exist, else the space's
// homepage.
func (r *exportRun) folderParent(ctx context.Context) (string, error) {
	parent := r.plan.parentTitle
	if parent == "" {
		return r.opts.Client.SpaceHome(ctx, r.spaceID)
	}

	page, err := r.opts.Client.FindPage(ctx, r.spaceID, parent)
	if err != nil {
		return "", err
	}

	if page == nil {
		return "", &ConfigError{Reason: fmt.Sprintf(
			"sync.confluence.parent %q is not a page in space %s", parent, r.syncSpace())}
	}

	return page.ID, nil
}

// createFolder creates the folder under the parent page, or at the top of
// the space, and writes its property. A dry run creates nothing.
func (r *exportRun) createFolder(ctx context.Context) error {
	if r.opts.DryRun {
		return nil
	}

	parentID := ""

	if r.plan.parentTitle != "" {
		var err error
		if parentID, err = r.folderParent(ctx); err != nil {
			return err
		}
	}

	folder, err := r.opts.Client.CreateFolder(ctx, &NewFolder{SpaceID: r.spaceID, ParentID: parentID, Title: r.plan.folder})

	var te *TitleError
	if errors.As(err, &te) {
		return &ConfigError{Reason: fmt.Sprintf(
			"folder title %q is taken elsewhere in space %s; set sync.confluence.folder to another",
			r.plan.folder, r.syncSpace())}
	}

	if err != nil {
		return err
	}

	value, err := json.Marshal(folderProperty{Repo: r.opts.Repository, Docz: r.opts.Version})
	if err != nil {
		return err
	}

	if err := r.opts.Client.SetProperty(ctx, FolderTarget(folder.ID), &Property{Key: propertyKey, Value: value}); err != nil {
		return err
	}

	r.useFolder(folder)

	return nil
}

// useFolder records the folder as the run's and the report's.
func (r *exportRun) useFolder(f *Folder) {
	r.folderID = f.ID
	r.report.Folder = f.Node()
}

// syncSpace is the configured space key, for messages.
func (r *exportRun) syncSpace() string { return r.plan.space }
