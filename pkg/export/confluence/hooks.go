package confluence

import "context"

// fireRequest reports a completed request to the context's hooks. Phase 4
// replaces this stub with the Hooks carried in the context.
func fireRequest(context.Context, string, string, int) {}
