package docx

import (
	"fmt"
	"strings"
)

// ReplaceXml will replace all occurrences of the placeholderKey with the given value.
// The function is synced with a mutex as it is not concurrency safe.
func (r *Replacer) ReplaceXml(placeholderKey string, value []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !strings.ContainsRune(placeholderKey, OpenDelimiter) ||
		!strings.ContainsRune(placeholderKey, CloseDelimiter) {
		placeholderKey = AddPlaceholderDelimiter(placeholderKey)
	}

	// find all occurrences of the placeholderKey inside r.placeholders
	found := false
	for i := 0; i < len(r.placeholders); i++ {
		placeholder := r.placeholders[i]

		if placeholder.Text(r.document) == placeholderKey {
			found = true

			// replace text of the placeholder'str first fragment with the actual value
			r.replaceParagraph(placeholder.Fragments[0], value)

			// the other fragments of the placeholder are cut, leaving only the value inside the document.
			for i := 1; i < len(placeholder.Fragments); i++ {
				r.cutFragment(placeholder.Fragments[i])
			}
		}
	}

	// all replacing actions might potentially screw up the XML structure
	// in order to capture this, all tags are re-validated after replacing a value
	if err := ValidatePositions(r.document, r.distinctRuns); err != nil {
		return fmt.Errorf("replace produced invalid result: %w", err)
	}

	if !found {
		return ErrPlaceholderNotFound
	}
	return nil
}



// replaceParagraph will replace the paragraph containing the whole fragment with the given value, adjusting all following
// fragments afterwards.
func (r *Replacer) replaceParagraph(fragment *PlaceholderFragment, value []byte) {
	var deltaLength int64

	closePreviousParagraph := []byte("</w:t></w:r></w:p>")
	openNewParagraph := []byte("<w:p><w:r><w:t>")
	value = append(append(closePreviousParagraph, value...), openNewParagraph...)

	docBytes := r.document
	valueLength := int64(len(string(value)))
	fragLength := fragment.EndPos() - fragment.StartPos()
	deltaLength = valueLength - fragLength

	// cut out the fragment entirely
	cutStart := fragment.StartPos()
	cutEnd := fragment.EndPos()
	docBytes = append(docBytes[:cutStart], docBytes[cutEnd:]...)

	// insert the value from the cut start position
	docBytes = append(docBytes[:cutStart], append(value, docBytes[cutStart:]...)...)

	// shift everything which is after the replaced value for this fragment
	fragment.ShiftReplace(deltaLength)

	r.document = docBytes
	r.ReplaceCount++
	r.BytesChanged += deltaLength
	r.shiftFollowingFragments(fragment, deltaLength)
}