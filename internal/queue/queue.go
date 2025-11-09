package queue

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"git.oxl.at/micro-queue/internal/u"
)

const (
	// The main data log file
	dataLogFileName = "data.log"
	// The metadata file (stores head/tail offsets)
	metaFileName = "meta.json"
	// Temporary file for atomic metadata writes
	metaTempFileName = "meta.json.tmp"
	// Temporary file for atomic compaction
	compactTempFileName = "data.log.tmp"
)

// meta holds the state of the queue's pointers into the data log.
type meta struct {
	// HeadOffset is the byte offset of the *next* item to be read.
	HeadOffset int64 `json:"head_offset"`
	// TailOffset is the byte offset of the *end* of the last item (i.e., where to write next).
	TailOffset int64 `json:"tail_offset"`
}

// PersistentQueue is a concurrent-safe, persistent, log-structured queue for 'string' items.
type PersistentQueue struct {
	mu sync.Mutex
	// The directory holding the queue files
	DirPath string
	// The data log file
	dataLog *os.File
	// In-memory state
	meta meta
}

// NewPersistentQueue creates or loads a persistent queue from the given directory.
func NewPersistentQueue(dirPath string) (*PersistentQueue, error) {
	// 1. Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create queue directory %s: %w", dirPath, err)
	}

	// 2. Load metadata, or create it if it doesn't exist
	metaPath := filepath.Join(dirPath, metaFileName)
	m := meta{}
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, we are a new queue.
			// 'm' is already zeroed (HeadOffset: 0, TailOffset: 0)
			u.LogDebug(fmt.Sprintf("No metadata file found at %s. Creating new queue.", metaPath))

		} else {
			// Another error (e.g., permissions)
			return nil, fmt.Errorf("failed to read metadata file %s: %w", metaPath, err)
		}

	} else {
		// File exists, unmarshal it
		if err := json.Unmarshal(metaBytes, &m); err != nil {
			return nil, fmt.Errorf("failed to parse metadata file %s: %w", metaPath, err)
		}
		u.LogDebug(fmt.Sprintf("Loaded metadata: Head=%d, Tail=%d", m.HeadOffset, m.TailOffset))
	}

	// 3. Open the data log file for reading and writing
	dataPath := filepath.Join(dirPath, dataLogFileName)
	dataLog, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open data log file %s: %w", dataPath, err)
	}

	q := &PersistentQueue{
		DirPath: dirPath,
		dataLog: dataLog,
		meta:    m,
	}
	q.writeMetadata()

	return q, nil
}

// writeMetadata atomically writes the current metadata to disk.
func (q *PersistentQueue) writeMetadata() error {
	// This is the atomic write sequence:
	// 1. Marshal current in-memory metadata
	metaBytes, err := json.Marshal(q.meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 2. Write to a temporary file
	tempPath := filepath.Join(q.DirPath, metaTempFileName)
	if err := os.WriteFile(tempPath, metaBytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp metadata file: %w", err)
	}

	// 3. Atomically rename the temp file to the real file
	metaPath := filepath.Join(q.DirPath, metaFileName)
	if err := os.Rename(tempPath, metaPath); err != nil {
		return fmt.Errorf("failed to atomically rename metadata file: %w", err)
	}

	return nil
}

// Enqueue adds an item to the back of the persistent queue.
func (q *PersistentQueue) Enqueue(item string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	itemBytes := []byte(item)
	itemLen := int64(len(itemBytes))

	// We use a simple length-prefixing format:
	// [8 bytes for length][N bytes of data]
	lenBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBuf, uint64(itemLen))

	// 1. Seek to the tail of the log
	// Note: We use Seek, not O_APPEND, to control the offset precisely.
	if _, err := q.dataLog.Seek(q.meta.TailOffset, io.SeekStart); err != nil {
		return fmt.Errorf("enqueue: failed to seek to tail offset %d: %w", q.meta.TailOffset, err)
	}

	// 2. Write the 8-byte length prefix
	if _, err := q.dataLog.Write(lenBuf); err != nil {
		return fmt.Errorf("enqueue: failed to write length prefix: %w", err)
	}

	// 3. Write the item data
	if _, err := q.dataLog.Write(itemBytes); err != nil {
		return fmt.Errorf("enqueue: failed to write item data: %w", err)
	}

	// 4. Update in-memory tail offset
	q.meta.TailOffset += 8 + itemLen

	// 5. Atomically write the new metadata to disk
	if err := q.writeMetadata(); err != nil {
		// This is tricky. The data is written, but the metadata failed.
		// A real implementation might retry or panic.
		// For now, we'll log and return the error.
		return fmt.Errorf("enqueue: data written but metadata update failed: %w", err)
	}

	return nil
}

// Dequeue removes and returns the item from the front of the queue.
func (q *PersistentQueue) Dequeue() (string, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 1. Check if queue is empty
	if q.meta.HeadOffset == q.meta.TailOffset {
		return "", false, nil // false = queue is empty
	}

	// 2. Seek to the head of the log to read the next item
	if _, err := q.dataLog.Seek(q.meta.HeadOffset, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("dequeue: failed to seek to head offset %d: %w", q.meta.HeadOffset, err)
	}

	// 3. Read the 8-byte length prefix
	lenBuf := make([]byte, 8)
	if _, err := io.ReadFull(q.dataLog, lenBuf); err != nil {
		return "", false, fmt.Errorf("dequeue: failed to read length prefix: %w", err)
	}
	itemLen := int64(binary.BigEndian.Uint64(lenBuf))

	// 4. Read the item data
	itemBytes := make([]byte, itemLen)
	if _, err := io.ReadFull(q.dataLog, itemBytes); err != nil {
		return "", false, fmt.Errorf("dequeue: failed to read item data: %w", err)
	}

	// 5. Update in-memory head offset
	q.meta.HeadOffset += 8 + itemLen

	// 6. Atomically write the new metadata to disk
	if err := q.writeMetadata(); err != nil {
		return "", false, fmt.Errorf("dequeue: data read but metadata update failed: %w", err)
	}

	return string(itemBytes), true, nil // true = item was found
}

// Len returns the current *byte size* of the active queue (not item count).
// An accurate item count would require iterating, which is slow.
func (q *PersistentQueue) Len() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.meta.TailOffset - q.meta.HeadOffset
}

// Compact rewrites the data log to remove "dead" (dequeued) space.
func (q *PersistentQueue) Compact() error {
	log.Println("Starting compaction...")
	q.mu.Lock()
	defer q.mu.Unlock()

	// 1. Check if compaction is needed
	if q.meta.HeadOffset == 0 {
		u.LogDebug("Compaction not needed (head is already at 0).")
		return nil
	}
	if q.meta.HeadOffset == q.meta.TailOffset {
		u.LogDebug("Compaction: Queue is empty, truncating files.")
		// Easiest case: queue is empty, just reset everything.
		if err := q.dataLog.Truncate(0); err != nil {
			return fmt.Errorf("compact: failed to truncate log: %w", err)
		}
		q.meta.HeadOffset = 0
		q.meta.TailOffset = 0
		return q.writeMetadata()
	}

	// 2. Create a new temporary log file
	tempPath := filepath.Join(q.DirPath, compactTempFileName)
	tempLog, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("compact: failed to create temp log: %w", err)
	}
	defer tempLog.Close() // Close on function exit

	// 3. Seek to the head of the *current* log
	if _, err := q.dataLog.Seek(q.meta.HeadOffset, io.SeekStart); err != nil {
		return fmt.Errorf("compact: failed to seek to head: %w", err)
	}

	// 4. Copy all *active* data (Head -> Tail) to the new temp log
	//    This will be a single, large copy.
	bytesToCopy := q.meta.TailOffset - q.meta.HeadOffset
	if _, err := io.CopyN(tempLog, q.dataLog, bytesToCopy); err != nil {
		return fmt.Errorf("compact: failed to copy data to temp log: %w", err)
	}

	// 5. Close the *current* data log file so we can rename over it
	if err := q.dataLog.Close(); err != nil {
		return fmt.Errorf("compact: failed to close old log: %w", err)
	}

	// 6. Atomically rename the new log to the main log
	dataPath := filepath.Join(q.DirPath, dataLogFileName)
	if err := os.Rename(tempPath, dataPath); err != nil {
		return fmt.Errorf("compact: failed to rename temp log: %w", err)
	}

	// 7. Re-open the main data log file (it's the new one now)
	q.dataLog, err = os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("compact: failed to re-open new log: %w", err)
	}

	// 8. Update in-memory metadata to reflect the new state (Head is 0, Tail is new size)
	u.LogDebug(fmt.Sprintf("Compaction complete. Old Head: %d. New Head: 0. New Tail: %d.", q.meta.HeadOffset, bytesToCopy))
	q.meta.HeadOffset = 0
	q.meta.TailOffset = bytesToCopy

	// 9. Write the new metadata to disk
	return q.writeMetadata()
}
