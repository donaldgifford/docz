// Package export runs docz-api's Confluence export of one repository
// (DESIGN-0021 §5): it reads the repository's stored rows in one snapshot,
// checks the server's allow-list, builds the repository's file tree in
// memory, runs pkg/export/confluence in the folder layout, and records what
// happened in confluence_syncs and confluence_pages. The queue's export
// task calls Service.Run; nothing here reads the disk or GitHub.
package export
