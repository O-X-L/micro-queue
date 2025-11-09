package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newTestQueue is a helper function to create a new persistent queue in a
// unique temporary directory for each test.
func newTestQueue(t *testing.T) *PersistentQueue {
	t.Helper()
	tempDir := t.TempDir()
	pq, err := NewPersistentQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create new test queue in %s: %v", tempDir, err)
	}
	return pq
}

// TestNewPersistentQueue_New checks that a new queue is created with empty metadata.
func TestNewPersistentQueue_New(t *testing.T) {
	pq := newTestQueue(t)

	if pq.meta.HeadOffset != 0 {
		t.Errorf("expected new queue head offset to be 0, got %d", pq.meta.HeadOffset)
	}
	if pq.meta.TailOffset != 0 {
		t.Errorf("expected new queue tail offset to be 0, got %d", pq.meta.TailOffset)
	}

	// Check that files were created
	metaPath := filepath.Join(pq.DirPath, metaFileName)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("meta.json file was not created")
	}
	dataPath := filepath.Join(pq.DirPath, dataLogFileName)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("data.log file was not created")
	}
}

// TestNewPersistentQueue_Load checks that an existing queue's metadata is loaded correctly.
func TestNewPersistentQueue_Load(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Manually create a meta.json file to simulate an existing queue
	existingMeta := meta{HeadOffset: 12, TailOffset: 36}
	metaBytes, _ := json.Marshal(existingMeta)
	metaPath := filepath.Join(tempDir, metaFileName)
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		t.Fatalf("Failed to write manual meta file: %v", err)
	}

	// 2. Create a dummy data.log
	dataPath := filepath.Join(tempDir, dataLogFileName)
	if _, err := os.Create(dataPath); err != nil {
		t.Fatalf("Failed to create dummy data log: %v", err)
	}

	// 3. Load the queue
	pq, err := NewPersistentQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to load existing queue: %v", err)
	}

	// 4. Check if the metadata was loaded
	if pq.meta.HeadOffset != existingMeta.HeadOffset {
		t.Errorf("expected loaded head offset to be %d, got %d", existingMeta.HeadOffset, pq.meta.HeadOffset)
	}
	if pq.meta.TailOffset != existingMeta.TailOffset {
		t.Errorf("expected loaded tail offset to be %d, got %d", existingMeta.TailOffset, pq.meta.TailOffset)
	}
}

// TestDequeue_Empty checks that dequeuing from an empty queue returns (false, nil).
func TestDequeue_Empty(t *testing.T) {
	pq := newTestQueue(t)

	job, ok, err := pq.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue from empty queue returned an error: %v", err)
	}
	if ok {
		t.Error("Dequeue from empty queue returned ok=true")
	}
	if job != "" {
		t.Errorf("Dequeue from empty queue returned a job: %s", job)
	}
}

// TestEnqueueDequeue_Simple checks the full cycle for a single item.
func TestEnqueueDequeue_Simple(t *testing.T) {
	pq := newTestQueue(t)
	jobIn := "my-test-job-1"

	// 1. Enqueue
	if err := pq.Enqueue(jobIn); err != nil {
		t.Fatalf("Enqueue returned an error: %v", err)
	}

	// Check internal state
	expectedOffset := int64(8 + len(jobIn)) // 8 bytes for length prefix
	if pq.meta.HeadOffset != 0 {
		t.Errorf("head offset should be 0 after enqueue, got %d", pq.meta.HeadOffset)
	}
	if pq.meta.TailOffset != expectedOffset {
		t.Errorf("tail offset should be %d after enqueue, got %d", expectedOffset, pq.meta.TailOffset)
	}

	// 2. Dequeue
	jobOut, ok, err := pq.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue returned an error: %v", err)
	}
	if !ok {
		t.Error("Dequeue returned ok=false")
	}
	if jobOut != jobIn {
		t.Errorf("dequeued job mismatch: expected '%s', got '%s'", jobIn, jobOut)
	}

	// Check internal state
	if pq.meta.HeadOffset != expectedOffset {
		t.Errorf("head offset should be %d after dequeue, got %d", expectedOffset, pq.meta.HeadOffset)
	}
	if pq.meta.HeadOffset != pq.meta.TailOffset {
		t.Error("head and tail offsets should match after queue is emptied")
	}

	// 3. Check if empty again
	_, ok, err = pq.Dequeue()
	if err != nil {
		t.Fatalf("Second dequeue returned an error: %v", err)
	}
	if ok {
		t.Error("Second dequeue returned ok=true")
	}
}

// TestEnqueueDequeue_FIFO checks that items are dequeued in First-In, First-Out order.
func TestEnqueueDequeue_FIFO(t *testing.T) {
	pq := newTestQueue(t)
	jobs := []string{"job1", "job-numero-2", "third-job"}

	for _, jobIn := range jobs {
		if err := pq.Enqueue(jobIn); err != nil {
			t.Fatalf("Enqueue failed for '%s': %v", jobIn, err)
		}
	}

	for i, expectedJob := range jobs {
		jobOut, ok, err := pq.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue #%d failed: %v", i, err)
		}
		if !ok {
			t.Fatalf("Dequeue #%d returned ok=false", i)
		}
		if jobOut != expectedJob {
			t.Errorf("FIFO order mismatch: expected '%s', got '%s'", expectedJob, jobOut)
		}
	}

	// Queue should be empty now
	if pq.meta.HeadOffset != pq.meta.TailOffset {
		t.Error("head and tail offsets should match after queue is emptied")
	}
}

// TestPersistence_Reload checks that queue state is preserved after "restarting".
func TestPersistence_Reload(t *testing.T) {
	tempDir := t.TempDir()

	// 1. First instance: Enqueue 3, Dequeue 1
	pq1, err := NewPersistentQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create queue 1: %v", err)
	}
	pq1.Enqueue("jobA")
	pq1.Enqueue("jobB")
	pq1.Enqueue("jobC")
	pq1.Dequeue() // Dequeues jobA

	// Grab state before "restart"
	expectedHead := pq1.meta.HeadOffset
	expectedTail := pq1.meta.TailOffset
	if err := pq1.dataLog.Close(); err != nil {
		t.Fatalf("Failed to close dataLog1: %v", err)
	}

	// 2. Second instance: Load the same directory
	pq2, err := NewPersistentQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to load queue 2: %v", err)
	}

	// Check if state was loaded
	if pq2.meta.HeadOffset != expectedHead {
		t.Errorf("reloaded head offset mismatch: expected %d, got %d", expectedHead, pq2.meta.HeadOffset)
	}
	if pq2.meta.TailOffset != expectedTail {
		t.Errorf("reloaded tail offset mismatch: expected %d, got %d", expectedTail, pq2.meta.TailOffset)
	}

	// 3. Dequeue remaining items
	jobB, okB, _ := pq2.Dequeue()
	if !okB || jobB != "jobB" {
		t.Errorf("expected to dequeue 'jobB', got '%s'", jobB)
	}
	jobC, okC, _ := pq2.Dequeue()
	if !okC || jobC != "jobC" {
		t.Errorf("expected to dequeue 'jobC', got '%s'", jobC)
	}

	// 4. Check if empty
	_, ok, _ := pq2.Dequeue()
	if ok {
		t.Error("queue was not empty after dequeuing all items")
	}
}

// TestCompact_DeadSpace checks that compaction removes dead space.
func TestCompact_DeadSpace(t *testing.T) {
	pq := newTestQueue(t)

	pq.Enqueue("job1") // 12 bytes
	pq.Enqueue("job2") // 12 bytes
	pq.Enqueue("job3") // 12 bytes
	pq.Enqueue("job4") // 12 bytes

	pq.Dequeue() // Dequeues job1
	pq.Dequeue() // Dequeues job2

	// State before compact: Head=24, Tail=48
	if pq.meta.HeadOffset != 24 {
		t.Fatalf("Expected head to be 24 before compact, got %d", pq.meta.HeadOffset)
	}

	// 1. Compact
	if err := pq.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// 2. Check new state
	if pq.meta.HeadOffset != 0 {
		t.Errorf("head offset should be 0 after compact, got %d", pq.meta.HeadOffset)
	}
	// Tail should be the size of the remaining 2 jobs
	expectedTail := int64((8 + len("job3")) + (8 + len("job4"))) // 12 + 12 = 24
	if pq.meta.TailOffset != expectedTail {
		t.Errorf("tail offset should be %d after compact, got %d", expectedTail, pq.meta.TailOffset)
	}

	// 3. Check that remaining jobs are intact
	job3, ok3, _ := pq.Dequeue()
	if !ok3 || job3 != "job3" {
		t.Errorf("expected 'job3' after compact, got '%s'", job3)
	}
	job4, ok4, _ := pq.Dequeue()
	if !ok4 || job4 != "job4" {
		t.Errorf("expected 'job4' after compact, got '%s'", job4)
	}

	// 4. Check if empty
	if pq.meta.HeadOffset != pq.meta.TailOffset {
		t.Error("queue not empty after dequeuing compacted items")
	}
}

// TestCompact_Empty checks that compacting an empty queue works.
func TestCompact_Empty(t *testing.T) {
	pq := newTestQueue(t)
	if err := pq.Compact(); err != nil {
		t.Fatalf("Compact on empty queue failed: %v", err)
	}
	if pq.meta.HeadOffset != 0 || pq.meta.TailOffset != 0 {
		t.Error("compacting an empty queue did not result in zero offsets")
	}

	// Also test compacting an empty-but-dirty queue
	pq.Enqueue("job1")
	pq.Dequeue()
	if err := pq.Compact(); err != nil {
		t.Fatalf("Compact on empty (dirty) queue failed: %v", err)
	}
	if pq.meta.HeadOffset != 0 || pq.meta.TailOffset != 0 {
		t.Error("compacting an empty (dirty) queue did not result in zero offsets")
	}
}

// TestCompact_Full checks that compacting a full queue (no dead space) works.
func TestCompact_Full(t *testing.T) {
	pq := newTestQueue(t)
	pq.Enqueue("job1")
	pq.Enqueue("job2")

	// State before compact: Head=0, Tail=24
	expectedTail := pq.meta.TailOffset

	if err := pq.Compact(); err != nil {
		// This should be a no-op and return nil
		t.Fatalf("Compact on full queue failed: %v", err)
	}

	if pq.meta.HeadOffset != 0 {
		t.Errorf("head offset should be 0, got %d", pq.meta.HeadOffset)
	}
	if pq.meta.TailOffset != expectedTail {
		t.Errorf("tail offset should be %d, got %d", expectedTail, pq.meta.TailOffset)
	}
}
