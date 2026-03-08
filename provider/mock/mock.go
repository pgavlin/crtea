// Package mock provides a mock provider.Provider for testing and demo purposes.
package mock

import (
	"fmt"
	"sync"
	"time"

	"github.com/pgavlin/crtea/provider"
)

type replyRecord struct {
	CommentID string
	Body      string
}

// Mock implements provider.Provider with in-memory canned data.
type Mock struct {
	mu           sync.Mutex
	user         string
	request      provider.ReviewRequest
	diff         string
	commits      []provider.Commit
	commitDiffs  map[string]string
	reviews      []provider.Review
	comments     []provider.Comment
	conversation []provider.ConversationComment

	// Mutable state for write operations
	submitted []provider.SubmitReviewRequest
	replies   []replyRecord
	posted    []provider.ConversationComment

	// Track last refresh point
	lastRefreshIdx int
}

// New creates a Mock pre-populated with realistic sample data.
func New() *Mock {
	now := time.Now().Add(-2 * time.Hour)

	return &Mock{
		user: "you",
		request: provider.ReviewRequest{
			ID:      "42",
			Title:   "Add user authentication middleware",
			Body:    "This PR adds JWT-based authentication middleware.\n\n- Token validation with expiry checking\n- Rate limiting per authenticated user\n- Unit tests for all auth flows",
			State:   "open",
			Author:  "alice",
			URL:     "https://github.com/example/repo/pull/42",
			BaseRef: "main",
			HeadRef: "feature/auth",
			HeadSHA: "abc1234",
		},
		diff: mockDiff,
		commits: []provider.Commit{
			{
				ID:      "abc1234",
				ShortID: "abc1234",
				Summary: "Add auth middleware and wire into server",
				Author:  "alice",
				Time:    now.Add(-10 * time.Minute),
			},
			{
				ID:      "def5678",
				ShortID: "def5678",
				Summary: "Add middleware unit tests",
				Author:  "alice",
				Time:    now.Add(-20 * time.Minute),
			},
		},
		commitDiffs: map[string]string{
			"abc1234": mockDiffCommit1,
			"def5678": mockDiffCommit2,
		},
		reviews: []provider.Review{
			{
				ExternalID: "r1",
				Author:     "bob",
				Body:       "A few things to address before merging.",
				State:      provider.ReviewRequestChanges,
				CreatedAt:  now.Add(30 * time.Minute),
			},
			{
				ExternalID: "r2",
				Author:     "carol",
				Body:       "Looking good overall, minor nits.",
				State:      provider.ReviewComment,
				CreatedAt:  now.Add(45 * time.Minute),
			},
		},
		comments: []provider.Comment{
			{
				ExternalID: "c1",
				Author:     "bob",
				Body:       "This should validate the token expiry before using it.",
				Path:       "auth/middleware.go",
				Line:       25,
				Side:       "new",
				CreatedAt:  now.Add(31 * time.Minute),
			},
			{
				ExternalID: "c2",
				Author:     "alice",
				Body:       "Good catch, I'll add that check.",
				Path:       "auth/middleware.go",
				Line:       25,
				Side:       "new",
				ReplyToID:  "c1",
				CreatedAt:  now.Add(40 * time.Minute),
			},
			{
				ExternalID: "c3",
				Author:     "bob",
				Body:       "Thanks! Also consider the clock skew case — tokens near expiry might fail intermittently.",
				Path:       "auth/middleware.go",
				Line:       25,
				Side:       "new",
				ReplyToID:  "c1",
				CreatedAt:  now.Add(50 * time.Minute),
			},
			{
				ExternalID: "c4",
				Author:     "carol",
				Body:       "Nit: consider extracting this into a helper function.",
				Path:       "auth/middleware.go",
				Line:       40,
				Side:       "new",
				CreatedAt:  now.Add(46 * time.Minute),
			},
			{
				ExternalID: "c5",
				Author:     "bob",
				Body:       "Was this intentional? The old handler had rate limiting.",
				Path:       "server.go",
				Line:       15,
				Side:       "old",
				CreatedAt:  now.Add(32 * time.Minute),
			},
			{
				ExternalID: "c6",
				Author:     "carol",
				Body:       "Typo in package doc.",
				Path:       "auth/middleware.go",
				Line:       3,
				Side:       "new",
				CreatedAt:  now.Add(47 * time.Minute),
				IsOutdated: true,
			},
		},
		conversation: []provider.ConversationComment{
			{
				ExternalID: "cc1",
				Author:     "alice",
				Body:       "Ready for review! This adds JWT-based auth middleware.",
				CreatedAt:  now,
			},
			{
				ExternalID: "cc2",
				Author:     "bob",
				Body:       "I'll take a look this afternoon.",
				CreatedAt:  now.Add(15 * time.Minute),
			},
			{
				ExternalID: "cc3",
				Author:     "carol",
				Body:       "Reviewing now.",
				CreatedAt:  now.Add(44 * time.Minute),
			},
		},
	}
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) GetAuthenticatedUser() (string, error) {
	return m.user, nil
}

func (m *Mock) GetReviewRequest(id string) (*provider.ReviewRequest, error) {
	return &m.request, nil
}

func (m *Mock) GetDiff(id string) (string, error) {
	return m.diff, nil
}

func (m *Mock) ListCommits(id string) ([]provider.Commit, error) {
	return m.commits, nil
}

func (m *Mock) GetCommitDiff(id string, commitID string) (string, error) {
	if d, ok := m.commitDiffs[commitID]; ok {
		return d, nil
	}
	return "", fmt.Errorf("commit %s not found", commitID)
}

func (m *Mock) ListReviews(id string) ([]provider.Review, error) {
	return m.reviews, nil
}

func (m *Mock) ListComments(id string) ([]provider.Comment, error) {
	return m.comments, nil
}

func (m *Mock) ListConversation(id string) ([]provider.ConversationComment, error) {
	return m.conversation, nil
}

func (m *Mock) SubmitReview(id string, review provider.SubmitReviewRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submitted = append(m.submitted, review)
	return nil
}

func (m *Mock) ReplyToComment(id string, commentID string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replies = append(m.replies, replyRecord{CommentID: commentID, Body: body})
	return nil
}

func (m *Mock) PostConversationComment(id string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cc := provider.ConversationComment{
		ExternalID: fmt.Sprintf("cc-local-%d", len(m.posted)+1),
		Author:     m.user,
		Body:       body,
		CreatedAt:  time.Now(),
	}
	m.posted = append(m.posted, cc)
	return nil
}

func (m *Mock) EditComment(id string, commentID string, body string) error {
	return nil
}

func (m *Mock) DeleteComment(id string, commentID string) error {
	return nil
}

func (m *Mock) ResolveThread(id string, threadID string) error {
	return nil
}

func (m *Mock) UnresolveThread(id string, threadID string) error {
	return nil
}

func (m *Mock) MarkReadyForReview(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.request.IsDraft = false
	return nil
}

func (m *Mock) UpdateReviewRequest(id string, title string, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.request.Title = title
	m.request.Body = body
	return nil
}

func (m *Mock) Seed(rr *provider.ReviewRequest, comments []provider.Comment, reviews []provider.Review, conv []provider.ConversationComment) {
	// No-op: Mock tracks refresh state internally via lastRefreshIdx.
}

func (m *Mock) Refresh(id string) (*provider.RefreshResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &provider.RefreshResult{
		Request: &m.request,
	}

	// Return any posted conversation comments since last refresh
	if len(m.posted) > m.lastRefreshIdx {
		result.NewConversation = m.posted[m.lastRefreshIdx:]
		m.lastRefreshIdx = len(m.posted)
	}

	return result, nil
}

const mockDiff = `diff --git a/auth/middleware.go b/auth/middleware.go
new file mode 100644
--- /dev/null
+++ b/auth/middleware.go
@@ -0,0 +1,52 @@
+// Package auth provides authentication middleware.
+//
+// It validates JWT tokens and extracts user identity.
+package auth
+
+import (
+	"context"
+	"fmt"
+	"net/http"
+	"strings"
+	"time"
+)
+
+// contextKey is an unexported type for context keys in this package.
+type contextKey string
+
+const userKey contextKey = "user"
+
+// Middleware returns an HTTP middleware that validates JWT tokens.
+func Middleware(secret []byte) func(http.Handler) http.Handler {
+	return func(next http.Handler) http.Handler {
+		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+			token := extractToken(r)
+			if token == "" {
+				http.Error(w, "missing auth token", http.StatusUnauthorized)
+				return
+			}
+
+			claims, err := validateToken(token, secret)
+			if err != nil {
+				http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
+				return
+			}
+
+			ctx := context.WithValue(r.Context(), userKey, claims.Subject)
+			next.ServeHTTP(w, r.WithContext(ctx))
+		})
+	}
+}
+
+// extractToken pulls the Bearer token from the Authorization header.
+func extractToken(r *http.Request) string {
+	auth := r.Header.Get("Authorization")
+	if !strings.HasPrefix(auth, "Bearer ") {
+		return ""
+	}
+	return strings.TrimPrefix(auth, "Bearer ")
+}
+
+// validateToken checks the JWT signature and claims.
+func validateToken(tokenStr string, secret []byte) (*Claims, error) {
+	_ = time.Now() // placeholder for expiry check
+	return &Claims{Subject: "user"}, nil
+}
diff --git a/server.go b/server.go
--- a/server.go
+++ b/server.go
@@ -10,20 +10,15 @@
 import (
 	"log"
 	"net/http"
-	"time"
+
+	"example.com/app/auth"
 )

-// rateLimiter tracks request rates per IP.
-var rateLimiter = make(map[string]time.Time)
-
 func main() {
 	mux := http.NewServeMux()
 	mux.HandleFunc("/api/health", healthHandler)
 	mux.HandleFunc("/api/data", dataHandler)

-	// Apply rate limiting
-	handler := withRateLimit(mux)
-
-	log.Println("starting server on :8080")
-	log.Fatal(http.ListenAndServe(":8080", handler))
+	handler := auth.Middleware([]byte("secret"))(mux)
+	log.Fatal(http.ListenAndServe(":8080", handler))
 }
diff --git a/auth/middleware_test.go b/auth/middleware_test.go
new file mode 100644
--- /dev/null
+++ b/auth/middleware_test.go
@@ -0,0 +1,35 @@
+package auth
+
+import (
+	"net/http"
+	"net/http/httptest"
+	"testing"
+)
+
+func TestMiddleware_NoToken(t *testing.T) {
+	handler := Middleware([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		t.Fatal("handler should not be called")
+	}))
+
+	req := httptest.NewRequest("GET", "/", nil)
+	rec := httptest.NewRecorder()
+	handler.ServeHTTP(rec, req)
+
+	if rec.Code != http.StatusUnauthorized {
+		t.Errorf("expected 401, got %d", rec.Code)
+	}
+}
+
+func TestMiddleware_ValidToken(t *testing.T) {
+	handler := Middleware([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		w.WriteHeader(http.StatusOK)
+	}))
+
+	req := httptest.NewRequest("GET", "/", nil)
+	req.Header.Set("Authorization", "Bearer valid-token")
+	rec := httptest.NewRecorder()
+	handler.ServeHTTP(rec, req)
+
+	if rec.Code != http.StatusOK {
+		t.Errorf("expected 200, got %d", rec.Code)
+	}
+}
`

// mockDiffCommit1 is the diff for commit abc1234: middleware + server wiring.
const mockDiffCommit1 = `diff --git a/auth/middleware.go b/auth/middleware.go
new file mode 100644
--- /dev/null
+++ b/auth/middleware.go
@@ -0,0 +1,52 @@
+// Package auth provides authentication middleware.
+//
+// It validates JWT tokens and extracts user identity.
+package auth
+
+import (
+	"context"
+	"fmt"
+	"net/http"
+	"strings"
+	"time"
+)
+
+// contextKey is an unexported type for context keys in this package.
+type contextKey string
+
+const userKey contextKey = "user"
+
+// Middleware returns an HTTP middleware that validates JWT tokens.
+func Middleware(secret []byte) func(http.Handler) http.Handler {
+	return func(next http.Handler) http.Handler {
+		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+			token := extractToken(r)
+			if token == "" {
+				http.Error(w, "missing auth token", http.StatusUnauthorized)
+				return
+			}
+
+			claims, err := validateToken(token, secret)
+			if err != nil {
+				http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
+				return
+			}
+
+			ctx := context.WithValue(r.Context(), userKey, claims.Subject)
+			next.ServeHTTP(w, r.WithContext(ctx))
+		})
+	}
+}
+
+// extractToken pulls the Bearer token from the Authorization header.
+func extractToken(r *http.Request) string {
+	auth := r.Header.Get("Authorization")
+	if !strings.HasPrefix(auth, "Bearer ") {
+		return ""
+	}
+	return strings.TrimPrefix(auth, "Bearer ")
+}
+
+// validateToken checks the JWT signature and claims.
+func validateToken(tokenStr string, secret []byte) (*Claims, error) {
+	_ = time.Now() // placeholder for expiry check
+	return &Claims{Subject: "user"}, nil
+}
diff --git a/server.go b/server.go
--- a/server.go
+++ b/server.go
@@ -10,20 +10,15 @@
 import (
 	"log"
 	"net/http"
-	"time"
+
+	"example.com/app/auth"
 )

-// rateLimiter tracks request rates per IP.
-var rateLimiter = make(map[string]time.Time)
-
 func main() {
 	mux := http.NewServeMux()
 	mux.HandleFunc("/api/health", healthHandler)
 	mux.HandleFunc("/api/data", dataHandler)

-	// Apply rate limiting
-	handler := withRateLimit(mux)
-
-	log.Println("starting server on :8080")
-	log.Fatal(http.ListenAndServe(":8080", handler))
+	handler := auth.Middleware([]byte("secret"))(mux)
+	log.Fatal(http.ListenAndServe(":8080", handler))
 }
`

// mockDiffCommit2 is the diff for commit def5678: unit tests.
const mockDiffCommit2 = `diff --git a/auth/middleware_test.go b/auth/middleware_test.go
new file mode 100644
--- /dev/null
+++ b/auth/middleware_test.go
@@ -0,0 +1,35 @@
+package auth
+
+import (
+	"net/http"
+	"net/http/httptest"
+	"testing"
+)
+
+func TestMiddleware_NoToken(t *testing.T) {
+	handler := Middleware([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		t.Fatal("handler should not be called")
+	}))
+
+	req := httptest.NewRequest("GET", "/", nil)
+	rec := httptest.NewRecorder()
+	handler.ServeHTTP(rec, req)
+
+	if rec.Code != http.StatusUnauthorized {
+		t.Errorf("expected 401, got %d", rec.Code)
+	}
+}
+
+func TestMiddleware_ValidToken(t *testing.T) {
+	handler := Middleware([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+		w.WriteHeader(http.StatusOK)
+	}))
+
+	req := httptest.NewRequest("GET", "/", nil)
+	req.Header.Set("Authorization", "Bearer valid-token")
+	rec := httptest.NewRecorder()
+	handler.ServeHTTP(rec, req)
+
+	if rec.Code != http.StatusOK {
+		t.Errorf("expected 200, got %d", rec.Code)
+	}
+}
`
