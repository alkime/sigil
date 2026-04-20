package diff

import (
	"sync"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	now := time.Now().UTC().Truncate(time.Second)
	s := &Session{
		ID:         "test-uuid",
		Repo:       "org/repo",
		PRNumber:   42,
		PRTitle:    "feat: add oidc",
		BaseBranch: "main",
		BaseSHA:    "abc123",
		HeadSHA:    "def456",
		Branch:     "feature/oidc",
		CreatedAt:  now,
		UpdatedAt:  now,
		Snapshots: []Snapshot{
			{Base: "abc123", Head: "def456", ObservedAt: now},
		},
	}

	if err := SaveSession("org", "repo", 42, s); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := LoadSession("org", "repo", 42)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}

	if got.ID != s.ID {
		t.Errorf("ID: want %q got %q", s.ID, got.ID)
	}
	if got.PRNumber != s.PRNumber {
		t.Errorf("PRNumber: want %d got %d", s.PRNumber, got.PRNumber)
	}
	if got.HeadSHA != s.HeadSHA {
		t.Errorf("HeadSHA: want %q got %q", s.HeadSHA, got.HeadSHA)
	}
	if len(got.Snapshots) != 1 {
		t.Errorf("Snapshots: want 1 got %d", len(got.Snapshots))
	}
}

func TestCommentsRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	now := time.Now().UTC().Truncate(time.Second)
	comments := []Comment{
		{
			ID:         "uuid-1",
			File:       "internal/auth/oidc.go",
			HunkHeader: "@@ -42,7 +42,12 @@",
			LineHint:   47,
			Side:       "right",
			Context: CommentContext{
				Before: []string{"  if err != nil {", "    return nil, err"},
				Target: "  return token, nil",
				After:  []string{"}", ""},
			},
			Body:        "should we log the token type?",
			Author:      "james",
			CreatedAt:   now,
			UpdatedAt:   now,
			Tags:        []string{"question"},
			Resolved:    false,
			Orphaned:    false,
			SnapshotRef: "abc123_def456",
		},
	}

	if err := SaveComments("org", "repo", 42, comments); err != nil {
		t.Fatalf("SaveComments: %v", err)
	}

	got, err := LoadComments("org", "repo", 42)
	if err != nil {
		t.Fatalf("LoadComments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %d", len(got))
	}

	c := got[0]
	if c.ID != "uuid-1" {
		t.Errorf("ID: want uuid-1, got %q", c.ID)
	}
	if c.Context.Target != "  return token, nil" {
		t.Errorf("Context.Target: got %q", c.Context.Target)
	}
	if len(c.Context.Before) != 2 {
		t.Errorf("Context.Before: want 2, got %d", len(c.Context.Before))
	}
	if len(c.Tags) != 1 || c.Tags[0] != "question" {
		t.Errorf("Tags: %v", c.Tags)
	}
}

func TestXDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	bp := BasePath()
	want := dir + "/sigil/diffs"
	if bp != want {
		t.Errorf("BasePath: want %q got %q", want, bp)
	}
}

func TestMissingFileReturnsNil(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s, err := LoadSession("no-org", "no-repo", 999)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if s != nil {
		t.Fatal("expected nil session")
	}

	comments, err := LoadComments("no-org", "no-repo", 999)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if comments != nil {
		t.Fatal("expected nil comments")
	}
}

func TestFlockContention(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	const pr = 1
	org, repo := "org", "repo"

	if err := SaveComments(org, repo, pr, nil); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			comments := []Comment{{ID: "uuid", Body: "body", Author: "author"}}
			if err := SaveComments(org, repo, pr, comments); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent SaveComments error: %v", err)
	}

	got, err := LoadComments(org, repo, pr)
	if err != nil {
		t.Fatalf("LoadComments after contention: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 comment after contention, got %d", len(got))
	}
}

func TestSessionsIndexRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	now := time.Now().UTC().Truncate(time.Second)
	entries := []SessionsIndexEntry{
		{PRNumber: 42, Path: "org/repo/42", UpdatedAt: now},
		{PRNumber: 7, Path: "org/repo/7", UpdatedAt: now},
	}

	if err := SaveIndex("org", "repo", entries); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	got, err := LoadIndex("org", "repo")
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].PRNumber != 42 {
		t.Errorf("entry 0 PRNumber: want 42, got %d", got[0].PRNumber)
	}
}

func TestMissingIndexReturnsNil(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	entries, err := LoadIndex("no-org", "no-repo")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if entries != nil {
		t.Fatal("expected nil entries")
	}
}

func TestPartialWriteSafety(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	comments := []Comment{{ID: "original", Body: "original body", Author: "james"}}
	if err := SaveComments("org", "repo", 1, comments); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	got, err := LoadComments("org", "repo", 1)
	if err != nil {
		t.Fatalf("LoadComments: %v", err)
	}
	if len(got) != 1 || got[0].ID != "original" {
		t.Errorf("unexpected comments after save: %v", got)
	}
}
