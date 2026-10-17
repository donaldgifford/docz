package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/internal/httpapi"
)

// The resource URI scheme (DESIGN-0023 §4): docz://<owner>/<name>/<type>/<id>.
const (
	resourceScheme = "docz://"
	mimeMarkdown   = "text/markdown"
	mimeJSON       = "application/json"
)

// builtinTypes are the six built-in type names, in registry order, each
// of which gets its own template beside the generic one.
var builtinTypes = []string{KindRFC, KindADR, KindDesign, KindImpl, KindInvestigation, KindRunbook}

// registerResources adds one template per built-in type and one generic
// (question 7): resources/list is the same for every caller, and the set
// of documents a caller may see is not, so the lists hold templates and
// list_docs names the documents.
func registerResources(srv *sdk.Server, t *tools) {
	for _, name := range builtinTypes {
		srv.AddResourceTemplate(&sdk.ResourceTemplate{
			Name:        "docz-" + name,
			Title:       strings.ToUpper(name[:1]) + name[1:] + " documents",
			Description: fmt.Sprintf("One %s document by repository and id, as markdown and as its typed fields.", name),
			URITemplate: resourceScheme + "{owner}/{name}/" + name + "/{id}",
			MIMEType:    mimeMarkdown,
		}, t.readResource)
	}
	srv.AddResourceTemplate(&sdk.ResourceTemplate{
		Name:        "docz-document",
		Title:       "Documents",
		Description: "One document of any type by repository, type, and id, as markdown and as its typed fields.",
		URITemplate: resourceScheme + "{owner}/{name}/{type}/{id}",
		MIMEType:    mimeMarkdown,
	}, t.readResource)
}

// readResource serves resources/read: the markdown as text/markdown and
// the typed fields as application/json. A URI naming a repository outside
// the caller's allowed set, a type the repository lacks, or a document it
// does not hold is not found, and the three are told apart no more than
// the REST API tells them apart.
func (t *tools) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	uri := req.Params.URI
	owner, name, typeToken, id, ok := parseResourceURI(uri)
	if !ok {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	repo, row, err := t.document(ctx, owner+"/"+name, id)
	if err != nil {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	// The URI's type must be the document's: a document is reachable under
	// its own type's URI and no other, as in the REST route.
	if !strings.EqualFold(typeToken, row.Document.Type) && !resolvesTo(ctx, t, repo.ID, typeToken, row.Document.Type) {
		return nil, sdk.ResourceNotFoundError(uri)
	}

	fields, err := json.Marshal(Fields(row.Document.Type, []byte(row.Document.RawMd)))
	if err != nil {
		return nil, fmt.Errorf("encoding fields: %w", err)
	}

	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{
			{URI: uri, MIMEType: mimeMarkdown, Text: row.Document.RawMd},
			{URI: uri, MIMEType: mimeJSON, Text: string(fields)},
		},
	}, nil
}

// resolvesTo reports whether token names canonical among the repository's
// types, so docz://o/n/inv/INV-0001 reads the investigation.
func resolvesTo(ctx context.Context, t *tools, repoID int64, token, canonical string) bool {
	types, err := t.store.GetDocTypesForRepo(ctx, repoID)
	if err != nil {
		return false
	}
	resolved, ok := httpapi.ResolveType(types, token)

	return ok && resolved == canonical
}

// parseResourceURI splits docz://owner/name/type/id. Every part is
// required and none may be empty; anything else is not a resource.
func parseResourceURI(uri string) (owner, name, typeToken, id string, ok bool) {
	rest, found := strings.CutPrefix(uri, resourceScheme)
	if !found {
		return "", "", "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	for _, p := range parts {
		if p == "" {
			return "", "", "", "", false
		}
	}

	return parts[0], parts[1], parts[2], parts[3], true
}
