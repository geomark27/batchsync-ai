package processor

import "batchsync-ai/internal/model"

// chunk splits entries into blocks of at most size elements.
func chunk(entries []model.LogEntry, size int) [][]model.LogEntry {
	if size <= 0 || len(entries) == 0 {
		return nil
	}

	blocks := make([][]model.LogEntry, 0, (len(entries)+size-1)/size)
	for size < len(entries) {
		entries, blocks = entries[size:], append(blocks, entries[:size])
	}
	return append(blocks, entries)
}
