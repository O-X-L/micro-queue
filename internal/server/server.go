package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"git.oxl.at/micro-queue/internal/config"
	"git.oxl.at/micro-queue/internal/queue"
	"git.oxl.at/micro-queue/internal/u"
)

func ServerHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "OXL-Micro-Queue")
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(next http.Handler, appConfig *config.AppConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("AUTH: Failed request. Missing Authorization header.")
			http.Error(w, "Missing Authorization header. Use 'Authorization: Bearer <token>'.", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			log.Println("AUTH: Failed request. Malformed Authorization header.")
			http.Error(w, "Malformed Authorization header. Use 'Authorization: Bearer <token>'.", http.StatusUnauthorized)
			return
		}
		token := parts[1]

		// validate
		perms, ok := appConfig.TokenMap[token]
		if !ok {
			log.Printf("AUTH: Failed request. Invalid token used: %s...", truncateToken(token))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// authorize
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) != 2 {
			next.ServeHTTP(w, r)
			return
		}
		action, queueName := pathParts[0], pathParts[1]

		var hasPermission bool
		switch action {
		case "in":
			if perms.IsAdmin || perms.CanPost[queueName] {
				hasPermission = true
			}

		case "out":
			if perms.IsAdmin || perms.CanGet[queueName] {
				hasPermission = true
			}

		case "compact":
			if perms.IsAdmin {
				hasPermission = true
			}

		default:
			hasPermission = false
		}

		if !hasPermission {
			log.Printf("AUTH: Denied. Token '%s' (%s) tried to '%s' on queue '%s'.", perms.Name, truncateToken(token), action, queueName)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		log.Printf("AUTH: Granted. Token '%s' performing '%s' on queue '%s'.", perms.Name, action, queueName)
		next.ServeHTTP(w, r)
	})
}

// for logging
func truncateToken(token string) string {
	if len(token) > 8 {
		return token[:8] + "..."
	}
	return token
}

func RootHandler(queues map[string]*queue.PersistentQueue, appConfig *config.AppConfig) http.HandlerFunc {
	// Build a simple string of queue names for the root path message
	var queueNames []string
	for _, q := range appConfig.Queues {
		queueNames = append(queueNames, q.Name)
	}
	availableQueues := strings.Join(queueNames, ", ")

	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the URL: /<action>/<queue_name>
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if len(parts) == 1 && parts[0] == "" {
			// Root path
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("Queue microservice running. Available queues: " + availableQueues))
			return
		}

		if len(parts) != 2 {
			http.Error(w, "Invalid path. Use format: /<action>/<queue_name>", http.StatusNotFound)
			return
		}

		action, queueName := parts[0], parts[1]

		// Find the queue
		pq, ok := queues[queueName]
		if !ok {
			http.Error(w, fmt.Sprintf("Queue not found: %s. Available: %s", queueName, availableQueues), http.StatusNotFound)
			return
		}

		// Handle OPTIONS requests for API description
		if r.Method == http.MethodOptions {
			handleOptions(w, action)
			return
		}

		// Route to the correct action handler
		// We know the user is authorized at this point, thanks to the middleware.
		switch action {
		case "in":
			handleEnqueue(w, r, pq)

		case "out":
			handleDequeue(w, r, pq)

		case "compact":
			handleCompact(w, r, pq)

		default:
			http.Error(w, "Invalid action. Use 'in', 'out', or 'compact'", http.StatusNotFound)
		}
	}
}

// handleOptions writes an API description for the requested action.
func handleOptions(w http.ResponseWriter, action string) {
	w.Header().Set("Content-Type", "text/plain")
	switch action {
	case "in":
		w.Header().Set("Allow", "POST, OPTIONS")
		w.Write([]byte("POST: Enqueues a new job. The raw request body (e.g., JSON) is treated as the job payload.\nRequires 'post' permission for this queue."))

	case "out":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.Write([]byte("GET: Dequeues, claims, and returns the next job payload from the queue.\nRequires 'get' permission for this queue."))

	case "compact":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.Write([]byte("GET: Triggers a background compaction of the queue's log file to remove old data.\nRequires 'admin' permission."))
	default:
		http.Error(w, "Unknown action", http.StatusNotFound)
	}
}

// handleEnqueue adds a new job (from the request body) to the queue.
func handleEnqueue(w http.ResponseWriter, r *http.Request, pq *queue.PersistentQueue) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method. Use POST to enqueue.", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read enqueue body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Cannot enqueue empty job", http.StatusBadRequest)
		return
	}

	// Enqueue the job. The body is treated as a string payload.
	if err := pq.Enqueue(string(body)); err != nil {
		log.Printf("ERROR: Failed to enqueue job: %v", err)
		http.Error(w, "Failed to enqueue job", http.StatusInternalServerError)
		return
	}

	msg := "Enqueued job"
	fmt.Fprintf(w, "%s\n", msg)
	u.LogDebug(fmt.Sprintf("%s (Queue: %s, Size: %d bytes)", msg, pq.DirPath, pq.Len()))
}

// handleDequeue removes and returns the next job from the queue.
func handleDequeue(w http.ResponseWriter, r *http.Request, pq *queue.PersistentQueue) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method. Use GET to dequeue.", http.StatusMethodNotAllowed)
		return
	}

	// Dequeue the job.
	job, ok, err := pq.Dequeue()
	if err != nil {
		log.Printf("ERROR: Failed to dequeue job: %v", err)
		http.Error(w, "Failed to dequeue job", http.StatusInternalServerError)
		return
	}

	if !ok {
		// The queue was empty
		http.Error(w, "Queue is empty", http.StatusNotFound)
		return
	}

	// We assume the payload is JSON, as per the use case.
	// This helps the client interpret the response correctly.
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(job))
	u.LogDebug(fmt.Sprintf("Dequeued job (Queue: %s, Size: %d bytes)", pq.DirPath, pq.Len()))
}

// handleCompact triggers a compaction for the queue.
func handleCompact(w http.ResponseWriter, r *http.Request, pq *queue.PersistentQueue) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method. Use GET to compact.", http.StatusMethodNotAllowed)
		return
	}

	if err := pq.Compact(); err != nil {
		log.Printf("ERROR: Failed to compact queue: %v", err)
		http.Error(w, "Failed to compact queue", http.StatusInternalServerError)
		return
	}
	u.LogDebug(fmt.Sprintf("Compaction successful. (Queue: %v)", pq.DirPath))
}
