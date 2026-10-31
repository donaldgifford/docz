package html

import "bytes"

// questions writes the open questions. Until they have a layout of their
// own the region is text.
func (*doc) questions(_ *bytes.Buffer, _ section) (bool, error) {
	return false, nil
}
