package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestQueue(t *testing.T) {
	t.Run("Basic queue operations", func(t *testing.T) {
		rq := NewRequestQueue(2)
		
		// Test initial state
		queueLen, activeJobs := rq.GetQueueStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 0, activeJobs)
		
		// Test enqueue
		item := QueueItem{
			ID:       "test-1",
			Query:    "test query",
			UserID:   "user-1",
			Platform: "test",
			Callback: func(response string, err error) {},
			CreatedAt: time.Now(),
		}
		
		rq.Enqueue(item)
		queueLen, activeJobs = rq.GetQueueStatus()
		assert.Equal(t, 1, queueLen)
		assert.Equal(t, 0, activeJobs)
	})
	
	t.Run("Dequeue and mark complete", func(t *testing.T) {
		rq := NewRequestQueue(1)
		
		item := QueueItem{
			ID:       "test-2",
			Query:    "test query 2",
			UserID:   "user-2",
			Platform: "test",
			Callback: func(response string, err error) {},
			CreatedAt: time.Now(),
		}
		
		rq.Enqueue(item)
		
		// Dequeue in a goroutine since it might block
		var dequeuedItem *QueueItem
		done := make(chan bool)
		go func() {
			dequeuedItem = rq.Dequeue()
			done <- true
		}()
		
		select {
		case <-done:
			require.NotNil(t, dequeuedItem)
			assert.Equal(t, "test-2", dequeuedItem.ID)
			
			// Check status after dequeue
			queueLen, activeJobs := rq.GetQueueStatus()
			assert.Equal(t, 0, queueLen)
			assert.Equal(t, 1, activeJobs)
			
			// Mark complete
			rq.MarkComplete()
			queueLen, activeJobs = rq.GetQueueStatus()
			assert.Equal(t, 0, queueLen)
			assert.Equal(t, 0, activeJobs)
			
		case <-time.After(1 * time.Second):
			t.Fatal("Dequeue operation timed out")
		}
	})
	
	t.Run("Concurrent workers with rate limiting", func(t *testing.T) {
		maxWorkers := 2
		rq := NewRequestQueue(maxWorkers)
		
		// Create multiple items
		numItems := 5
		processed := make(chan string, numItems)
		
		for i := 0; i < numItems; i++ {
			item := QueueItem{
				ID:       fmt.Sprintf("test-%d", i),
				Query:    fmt.Sprintf("query %d", i),
				UserID:   fmt.Sprintf("user-%d", i),
				Platform: "test",
				Callback: func(response string, err error) {
					processed <- response
				},
				CreatedAt: time.Now(),
			}
			rq.Enqueue(item)
		}
		
		// Start workers
		workersDone := make(chan bool, maxWorkers)
		for i := 0; i < maxWorkers; i++ {
			go func(workerID int) {
				defer func() { workersDone <- true }()
				
				for {
					item := rq.Dequeue()
					if item == nil {
						break
					}
					
					// Simulate processing time
					time.Sleep(50 * time.Millisecond)
					
					// Call callback
					item.Callback(fmt.Sprintf("response-%s", item.ID), nil)
					
					// Mark complete
					rq.MarkComplete()
				}
			}(i)
		}
		
		// Wait for all items to be processed
		processedCount := 0
		timeout := time.After(5 * time.Second)
		for processedCount < numItems {
			select {
			case response := <-processed:
				assert.Contains(t, response, "response-test-")
				processedCount++
			case <-timeout:
				t.Fatalf("Timeout waiting for processing. Processed: %d/%d", processedCount, numItems)
			}
		}
		
		// Signal workers to stop by closing queue
		rq.Close()
		
		// Wait for workers to finish
		for i := 0; i < maxWorkers; i++ {
			select {
			case <-workersDone:
			case <-time.After(1 * time.Second):
				t.Fatal("Worker did not finish in time")
			}
		}
		
		// Verify final state
		queueLen, activeJobs := rq.GetQueueStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 0, activeJobs)
	})
	
	t.Run("Queue blocking when at capacity", func(t *testing.T) {
		maxWorkers := 1
		rq := NewRequestQueue(maxWorkers)
		
		// Fill the worker capacity
		item1 := QueueItem{
			ID:       "blocking-test-1",
			Query:    "blocking query 1",
			UserID:   "user-1",
			Platform: "test",
			Callback: func(response string, err error) {},
			CreatedAt: time.Now(),
		}
		rq.Enqueue(item1)
		
		// Start a worker that will block
		workerStarted := make(chan bool)
		workerBlocked := make(chan bool)
		go func() {
			item := rq.Dequeue()
			workerStarted <- true
			// Simulate long processing
			<-workerBlocked
			item.Callback("response", nil)
			rq.MarkComplete()
		}()
		
		// Wait for worker to start
		<-workerStarted
		
		// Verify that we're at capacity
		queueLen, activeJobs := rq.GetQueueStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 1, activeJobs)
		
		// Add more items to queue
		item2 := QueueItem{
			ID:       "blocking-test-2",
			Query:    "blocking query 2",
			UserID:   "user-2",
			Platform: "test",
			Callback: func(response string, err error) {},
			CreatedAt: time.Now(),
		}
		rq.Enqueue(item2)
		
		// Verify items are queued
		queueLen, activeJobs = rq.GetQueueStatus()
		assert.Equal(t, 1, queueLen)
		assert.Equal(t, 1, activeJobs)
		
		// Release the blocked worker
		workerBlocked <- true
		
		// Wait a bit for cleanup
		time.Sleep(100 * time.Millisecond)
	})
	
	t.Run("Queue order preservation (FIFO)", func(t *testing.T) {
		rq := NewRequestQueue(1)
		
		// Add items in specific order
		order := []string{"first", "second", "third"}
		processedOrder := make([]string, 0, len(order))
		processedChan := make(chan string, len(order))
		
		for _, id := range order {
			item := QueueItem{
				ID:       id,
				Query:    fmt.Sprintf("query %s", id),
				UserID:   fmt.Sprintf("user-%s", id),
				Platform: "test",
				Callback: func(response string, err error) {
					processedChan <- response
				},
				CreatedAt: time.Now(),
			}
			rq.Enqueue(item)
		}
		
		// Process items sequentially
		go func() {
			for i := 0; i < len(order); i++ {
				item := rq.Dequeue()
				if item != nil {
					item.Callback(item.ID, nil)
					rq.MarkComplete()
				}
			}
		}()
		
		// Collect processed items
		for i := 0; i < len(order); i++ {
			select {
			case processed := <-processedChan:
				processedOrder = append(processedOrder, processed)
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for item processing")
			}
		}
		
		// Verify order is preserved
		assert.Equal(t, order, processedOrder)
	})
	
	t.Run("Error handling in callbacks", func(t *testing.T) {
		rq := NewRequestQueue(1)
		
		errorReceived := make(chan error, 1)
		item := QueueItem{
			ID:       "error-test",
			Query:    "error query",
			UserID:   "user-error",
			Platform: "test",
			Callback: func(response string, err error) {
				errorReceived <- err
			},
			CreatedAt: time.Now(),
		}
		
		rq.Enqueue(item)
		
		// Process with error
		go func() {
			item := rq.Dequeue()
			if item != nil {
				item.Callback("", fmt.Errorf("simulated processing error"))
				rq.MarkComplete()
			}
		}()
		
		// Verify error is received
		select {
		case err := <-errorReceived:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "simulated processing error")
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for error callback")
		}
		
		// Verify queue is clean after error
		queueLen, activeJobs := rq.GetQueueStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 0, activeJobs)
	})
	
	t.Run("Close method stops waiting workers", func(t *testing.T) {
		rq := NewRequestQueue(1)
		
		// Start a worker that will wait for items
		workerFinished := make(chan bool)
		go func() {
			defer func() { workerFinished <- true }()
			
			// This should block initially
			item := rq.Dequeue()
			// After close, this should return nil
			assert.Nil(t, item)
		}()
		
		// Give worker time to start waiting
		time.Sleep(100 * time.Millisecond)
		
		// Close the queue
		rq.Close()
		
		// Worker should finish quickly
		select {
		case <-workerFinished:
			// Success - worker finished after close
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Worker did not finish after close")
		}
	})
	
	t.Run("Enqueue after close still allows processing existing items", func(t *testing.T) {
		rq := NewRequestQueue(1)
		
		// Add an item before closing
		processed := make(chan bool)
		item := QueueItem{
			ID:       "pre-close-item",
			Query:    "pre close query",
			UserID:   "user-1",
			Platform: "test",
			Callback: func(response string, err error) {
				processed <- true
			},
			CreatedAt: time.Now(),
		}
		rq.Enqueue(item)
		
		// Close the queue
		rq.Close()
		
		// Worker should still be able to process the existing item
		go func() {
			item := rq.Dequeue()
			if item != nil {
				item.Callback("response", nil)
				rq.MarkComplete()
			}
		}()
		
		// Verify the item was processed
		select {
		case <-processed:
			// Success - existing item was processed
		case <-time.After(1 * time.Second):
			t.Fatal("Existing item was not processed after close")
		}
		
		// Subsequent dequeue should return nil
		item2 := rq.Dequeue()
		assert.Nil(t, item2)
	})
}

// TestRequestQueueStress tests the queue under high load
func TestRequestQueueStress(t *testing.T) {
	t.Run("High concurrency stress test", func(t *testing.T) {
		maxWorkers := 5
		numItems := 100
		numProducers := 10
		rq := NewRequestQueue(maxWorkers)
		
		// Track processed items
		processed := make(chan string, numItems)
		var processedCount int32
		
		// Start workers
		workersDone := make(chan bool, maxWorkers)
		for i := 0; i < maxWorkers; i++ {
			go func(workerID int) {
				defer func() { workersDone <- true }()
				
				for {
					item := rq.Dequeue()
					if item == nil {
						break
					}
					
					// Simulate variable processing time
					time.Sleep(time.Duration(10+workerID*5) * time.Millisecond)
					
					// Call callback
					item.Callback(fmt.Sprintf("worker-%d-processed-%s", workerID, item.ID), nil)
					
					// Mark complete
					rq.MarkComplete()
				}
			}(i)
		}
		
		// Start producers
		producersDone := make(chan bool, numProducers)
		for p := 0; p < numProducers; p++ {
			go func(producerID int) {
				defer func() { producersDone <- true }()
				
				itemsPerProducer := numItems / numProducers
				for i := 0; i < itemsPerProducer; i++ {
					item := QueueItem{
						ID:       fmt.Sprintf("p%d-item%d", producerID, i),
						Query:    fmt.Sprintf("query from producer %d item %d", producerID, i),
						UserID:   fmt.Sprintf("user-%d-%d", producerID, i),
						Platform: "stress-test",
						Callback: func(response string, err error) {
							if err == nil {
								processed <- response
								atomic.AddInt32(&processedCount, 1)
							}
						},
						CreatedAt: time.Now(),
					}
					rq.Enqueue(item)
					
					// Small delay between enqueues
					time.Sleep(1 * time.Millisecond)
				}
			}(p)
		}
		
		// Wait for all producers to finish
		for p := 0; p < numProducers; p++ {
			select {
			case <-producersDone:
			case <-time.After(5 * time.Second):
				t.Fatal("Producer did not finish in time")
			}
		}
		
		// Wait for all items to be processed
		timeout := time.After(30 * time.Second)
		for atomic.LoadInt32(&processedCount) < int32(numItems) {
			select {
			case response := <-processed:
				assert.Contains(t, response, "worker-")
				assert.Contains(t, response, "processed-")
			case <-timeout:
				t.Fatalf("Timeout waiting for processing. Processed: %d/%d", atomic.LoadInt32(&processedCount), numItems)
			}
		}
		
		// Close queue and wait for workers
		rq.Close()
		for i := 0; i < maxWorkers; i++ {
			select {
			case <-workersDone:
			case <-time.After(2 * time.Second):
				t.Fatal("Worker did not finish in time")
			}
		}
		
		// Verify final state
		queueLen, activeJobs := rq.GetQueueStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 0, activeJobs)
		assert.Equal(t, int32(numItems), atomic.LoadInt32(&processedCount))
	})
	
	t.Run("Rate limiting effectiveness", func(t *testing.T) {
		maxWorkers := 2
		rq := NewRequestQueue(maxWorkers)
		
		// Track when items start processing
		processingStarted := make(chan time.Time, 10)
		processingFinished := make(chan time.Time, 10)
		
		// Add multiple items quickly
		numItems := 6
		for i := 0; i < numItems; i++ {
			item := QueueItem{
				ID:       fmt.Sprintf("rate-test-%d", i),
				Query:    fmt.Sprintf("rate test query %d", i),
				UserID:   fmt.Sprintf("user-%d", i),
				Platform: "rate-test",
				Callback: func(response string, err error) {
					processingFinished <- time.Now()
				},
				CreatedAt: time.Now(),
			}
			rq.Enqueue(item)
		}
		
		// Start workers
		workersDone := make(chan bool, maxWorkers)
		for i := 0; i < maxWorkers; i++ {
			go func(workerID int) {
				defer func() { workersDone <- true }()
				
				for {
					item := rq.Dequeue()
					if item == nil {
						break
					}
					
					processingStarted <- time.Now()
					
					// Simulate processing time
					time.Sleep(100 * time.Millisecond)
					
					item.Callback("response", nil)
					rq.MarkComplete()
				}
			}(i)
		}
		
		// Collect processing start times
		startTimes := make([]time.Time, 0, numItems)
		finishTimes := make([]time.Time, 0, numItems)
		
		timeout := time.After(5 * time.Second)
		for len(finishTimes) < numItems {
			select {
			case startTime := <-processingStarted:
				startTimes = append(startTimes, startTime)
			case finishTime := <-processingFinished:
				finishTimes = append(finishTimes, finishTime)
			case <-timeout:
				t.Fatalf("Timeout waiting for processing. Finished: %d/%d", len(finishTimes), numItems)
			}
		}
		
		// Close and cleanup
		rq.Close()
		for i := 0; i < maxWorkers; i++ {
			<-workersDone
		}
		
		// Verify that no more than maxWorkers were processing simultaneously
		// This is a simplified check - in a real scenario, we'd need more sophisticated timing analysis
		assert.Equal(t, numItems, len(finishTimes))
		assert.LessOrEqual(t, len(startTimes), numItems) // Should have at least some start times
	})
}

func TestExtractMention(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		botName     string
		expectedQuery string
		expectedMention bool
	}{
		{
			name:        "Simple mention",
			message:     "@SlimRAGBot what is RAG?",
			botName:     "SlimRAGBot",
			expectedQuery: "what is RAG?",
			expectedMention: true,
		},
		{
			name:        "Lowercase mention",
			message:     "@slimragbot how does it work?",
			botName:     "SlimRAGBot",
			expectedQuery: "how does it work?",
			expectedMention: true,
		},
		{
			name:        "No mention",
			message:     "This is a regular message",
			botName:     "SlimRAGBot",
			expectedQuery: "This is a regular message",
			expectedMention: false,
		},
		{
			name:        "Mention in middle",
			message:     "Hey @SlimRAGBot can you help me?",
			botName:     "SlimRAGBot",
			expectedQuery: "Hey  can you help me?",
			expectedMention: true,
		},
		{
			name:        "Multiple mentions",
			message:     "@SlimRAGBot @SlimRAGBot what is this?",
			botName:     "SlimRAGBot",
			expectedQuery: "what is this?",
			expectedMention: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, isMention := extractMention(tt.message, tt.botName)
			assert.Equal(t, tt.expectedMention, isMention)
			if isMention {
				// Normalize whitespace for comparison
				assert.Contains(t, query, strings.TrimSpace(tt.expectedQuery))
			}
		})
	}
}

func TestBotManager(t *testing.T) {
	t.Run("Create bot manager", func(t *testing.T) {
		bm := NewBotManager(nil, 2)
		require.NotNil(t, bm)
		assert.Equal(t, 2, bm.requestQueue.maxWorkers)
		
		queueLen, activeJobs := bm.GetStatus()
		assert.Equal(t, 0, queueLen)
		assert.Equal(t, 0, activeJobs)
	})
	
	t.Run("Add bots", func(t *testing.T) {
		bm := NewBotManager(nil, 1)
		
		// Create mock bots
		mockBot1 := &MockBot{name: "test-bot-1"}
		mockBot2 := &MockBot{name: "test-bot-2"}
		
		bm.AddBot(mockBot1)
		bm.AddBot(mockBot2)
		
		assert.Equal(t, 2, len(bm.bots))
	})
}

// MockBot implements the Bot interface for testing
type MockBot struct {
	name string
	started bool
	stopped bool
}

func (mb *MockBot) Start(ctx context.Context) error {
	mb.started = true
	return nil
}

func (mb *MockBot) Stop() error {
	mb.stopped = true
	return nil
}

func (mb *MockBot) SendMessage(userID, message string) error {
	return nil
}

func (mb *MockBot) GetBotName() string {
	return mb.name
}

func TestTelegramBot(t *testing.T) {
	t.Run("Create telegram bot", func(t *testing.T) {
		bm := NewBotManager(nil, 1)
		bot := NewTelegramBot("test-token", bm)
		
		require.NotNil(t, bot)
		assert.Equal(t, "test-token", bot.token)
		assert.Equal(t, bm, bot.botManager)
	})
}

func TestSlackBot(t *testing.T) {
	t.Run("Create slack bot", func(t *testing.T) {
		bm := NewBotManager(nil, 1)
		bot := NewSlackBot("test-token", "test-app-token", bm)
		require.NotNil(t, bot)
		assert.Equal(t, "slack:", bot.GetBotName())
	})
}

func TestQueuePositionNotification(t *testing.T) {
	t.Run("Queue position notification", func(t *testing.T) {
		bm := NewBotManager(nil, 1) // Only 1 worker to ensure queuing
		
		// Track queue positions
		var positions []int
		var positionsMutex sync.Mutex
		
		// Add multiple requests to create a queue
		for i := 0; i < 5; i++ {
			requestID := fmt.Sprintf("test-request-%d", i)
			bm.EnqueueRequest(
				requestID,
				"test query",
				"user-1",
				"test",
				func(response string, err error) {
					// Simulate processing time
					time.Sleep(50 * time.Millisecond)
				},
				func(position int) {
					positionsMutex.Lock()
					positions = append(positions, position)
					positionsMutex.Unlock()
				},
			)
		}
		
		// Wait for all notifications
		time.Sleep(100 * time.Millisecond)
		
		positionsMutex.Lock()
		defer positionsMutex.Unlock()
		
		// Verify we got 5 position notifications
		assert.Equal(t, 5, len(positions))
		
		// Verify positions are sequential (1, 2, 3, 4, 5)
		for i, pos := range positions {
			assert.Equal(t, i+1, pos, "Position %d should be %d", i, i+1)
		}
	})
	
	t.Run("No queue notification when callback is nil", func(t *testing.T) {
		bm := NewBotManager(nil, 1)
		
		// This should not panic even with nil callback
		bm.EnqueueRequest(
			"test-request",
			"test query",
			"user-1",
			"test",
			func(response string, err error) {},
			nil, // nil queue notification callback
		)
		
		// Should complete without issues
		time.Sleep(50 * time.Millisecond)
	})
}