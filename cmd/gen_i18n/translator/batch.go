package translator

// Batch partitions entries into groups where each group has at most maxEntries
// entries and at most maxChars total source bytes. A single entry that exceeds
// maxChars is placed alone in its own batch rather than dropped — the caller
// decides how to handle oversized payloads.
func Batch(entries []Entry, maxEntries, maxChars int) [][]Entry {
	if len(entries) == 0 {
		return nil
	}
	var batches [][]Entry
	var cur []Entry
	curChars := 0

	for _, e := range entries {
		n := entryChars(e)
		if len(cur) > 0 && (len(cur) >= maxEntries || curChars+n > maxChars) {
			batches = append(batches, cur)
			cur = nil
			curChars = 0
		}
		cur = append(cur, e)
		curChars += n
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// entryChars returns the total byte length of all source cell values in e.
// Byte length (not rune count) is intentional: it is a conservative upper
// bound that keeps LLM context payloads safely within limits even for
// multi-byte UTF-8 text.
func entryChars(e Entry) int {
	n := 0
	for _, v := range e.Cells {
		n += len(v)
	}
	return n
}
