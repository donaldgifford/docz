package document

import "time"

// timeNow is a package-level variable for the current time function.
// Tests can override this to produce deterministic output.
var timeNow = time.Now
