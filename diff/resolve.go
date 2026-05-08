package diff

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ResolveOpts configures the Resolve call.
type ResolveOpts struct {
	SessionID    string
	IncludeDraft bool
	CWD          string

	// Local enables no-PR mode: diff HEAD against an auto-detected (or
	// explicit) base ref, persist locally, no GitHub interaction.
	Local bool
	// BaseRef overrides the auto-detected default branch for Local mode.
	// When non-empty it implies Local=true.
	BaseRef string
}

// PRCandidate is a PR found during auto-detection across worktrees.
type PRCandidate struct {
	WorktreePath string
	Branch       string
	PRTitle      string
	BaseRefName  string
	PRNumber     int
	Repo         string
}

// ErrPickerNeeded is returned when multiple distinct PRs are found across worktrees.
type ErrPickerNeeded struct {
	Candidates []PRCandidate
}

func (e *ErrPickerNeeded) Error() string { return "multiple PRs found — picker required" }

type snapshotMeta struct {
	ObservedAt        time.Time `yaml:"observed_at"`
	Branch            string    `yaml:"branch"`
	HeadCommitMessage string    `yaml:"head_commit_message"`
}

// Resolve finds or creates a diff session from opts, returning the session,
// its latest diff, and the absolute worktree path (empty when unavailable).
func Resolve(ctx context.Context, opts ResolveOpts) (*Session, *ParsedDiff, string, error) {
	if opts.SessionID != "" {
		return loadSessionByID(ctx, opts)
	}
	if opts.Local || opts.BaseRef != "" {
		return resolveLocal(ctx, opts)
	}
	return autoDetect(ctx, opts)
}

// ResolvePicked resolves a specific PR after the user has chosen one from the
// picker. Use this instead of Resolve to avoid re-triggering auto-detection
// (which would return ErrPickerNeeded again for the same multi-PR situation).
func ResolvePicked(ctx context.Context, c PRCandidate) (*Session, *ParsedDiff, string, error) {
	parts := strings.SplitN(c.Repo, "/", 2)
	if len(parts) != 2 {
		return nil, nil, "", fmt.Errorf("invalid repo %q in picked candidate", c.Repo)
	}
	return resolveCandidate(ctx, c, parts[0], parts[1])
}

func autoDetect(ctx context.Context, opts ResolveOpts) (*Session, *ParsedDiff, string, error) {
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, nil, "", fmt.Errorf("get working directory: %w", err)
		}
	}

	worktrees, err := listWorktrees(ctx, cwd)
	if err != nil {
		return nil, nil, "", fmt.Errorf("list worktrees: %w", err)
	}

	_, owner, repoName, err := OriginRemote(ctx, cwd)
	if err != nil {
		return nil, nil, "", fmt.Errorf("origin remote: %w", err)
	}
	repo := owner + "/" + repoName

	seen := make(map[int]PRCandidate)
	for _, wt := range worktrees {
		if wt.Branch == "" || wt.IsDetached {
			continue
		}
		prs, ghErr := GHPRListByHead(ctx, repo, wt.Branch, opts.IncludeDraft)
		if ghErr != nil {
			continue
		}
		for _, pr := range prs {
			if _, ok := seen[pr.Number]; !ok {
				seen[pr.Number] = PRCandidate{
					WorktreePath: wt.Path,
					Branch:       wt.Branch,
					PRTitle:      pr.Title,
					BaseRefName:  pr.BaseRefName,
					PRNumber:     pr.Number,
					Repo:         repo,
				}
			}
		}
	}

	candidates := make([]PRCandidate, 0, len(seen))
	for _, c := range seen {
		candidates = append(candidates, c)
	}

	if len(candidates) == 0 {
		return nil, nil, "", fmt.Errorf("no open PRs found across worktrees — create one with: gh pr create, or run with --local to review against the default branch")
	}
	if len(candidates) > 1 {
		return nil, nil, "", &ErrPickerNeeded{Candidates: candidates}
	}

	return resolveCandidate(ctx, candidates[0], owner, repoName)
}

func resolveCandidate(ctx context.Context, c PRCandidate, org, repoName string) (*Session, *ParsedDiff, string, error) {
	headSHA, err := revParse(ctx, c.WorktreePath, "HEAD")
	if err != nil {
		return nil, nil, "", fmt.Errorf("get head SHA: %w", err)
	}

	key := PRSessionKey(c.PRNumber)
	session, err := LoadSession(org, repoName, key)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load session: %w", err)
	}

	if session != nil {
		if session.HeadSHA == headSHA {
			pd, pdErr := loadLatestSnapshotDiff(session, org, repoName)
			if pdErr != nil {
				return nil, nil, "", pdErr
			}
			return session, pd, c.WorktreePath, nil
		}
		return captureNewSnapshotPR(ctx, session, c, org, repoName, headSHA)
	}

	return createSession(ctx, c, org, repoName, headSHA)
}

func createSession(ctx context.Context, c PRCandidate, org, repoName, headSHA string) (*Session, *ParsedDiff, string, error) {
	baseSHA := getBaseSHA(ctx, c.WorktreePath, c.BaseRefName)
	now := time.Now().UTC()

	session := &Session{
		ID:         newUUID(),
		Repo:       c.Repo,
		Kind:       SessionKindPR,
		PRNumber:   c.PRNumber,
		PRTitle:    c.PRTitle,
		BaseBranch: c.BaseRefName,
		BaseSHA:    baseSHA,
		HeadSHA:    headSHA,
		Branch:     c.Branch,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	pd, err := capturePRSnapshot(ctx, session, c, org, repoName, baseSHA, headSHA)
	if err != nil {
		return nil, nil, "", err
	}

	key := PRSessionKey(c.PRNumber)
	if err := SaveSession(org, repoName, key, session); err != nil {
		return nil, nil, "", fmt.Errorf("save session: %w", err)
	}
	if err := upsertIndex(org, repoName, session); err != nil {
		return nil, nil, "", fmt.Errorf("upsert index: %w", err)
	}

	return session, pd, c.WorktreePath, nil
}

func captureNewSnapshotPR(ctx context.Context, session *Session, c PRCandidate, org, repoName, headSHA string) (*Session, *ParsedDiff, string, error) {
	pd, err := capturePRSnapshot(ctx, session, c, org, repoName, session.BaseSHA, headSHA)
	if err != nil {
		return nil, nil, "", err
	}

	session.HeadSHA = headSHA
	session.UpdatedAt = time.Now().UTC()

	key := PRSessionKey(c.PRNumber)
	if err := SaveSession(org, repoName, key, session); err != nil {
		return nil, nil, "", fmt.Errorf("save session: %w", err)
	}
	if err := upsertIndex(org, repoName, session); err != nil {
		return nil, nil, "", fmt.Errorf("upsert index: %w", err)
	}

	return session, pd, c.WorktreePath, nil
}

func capturePRSnapshot(ctx context.Context, session *Session, c PRCandidate, org, repoName, baseSHA, headSHA string) (*ParsedDiff, error) {
	diffBytes, err := GHPRDiff(ctx, c.Repo, c.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("gh pr diff: %w", err)
	}
	return writeSnapshot(ctx, session, c.WorktreePath, c.Branch, PRSessionKey(c.PRNumber), org, repoName, baseSHA, headSHA, diffBytes)
}

// resolveLocal handles the no-PR path: --local with optional --base override.
// Auto-detects the default branch via DefaultBranch when BaseRef is empty.
func resolveLocal(ctx context.Context, opts ResolveOpts) (*Session, *ParsedDiff, string, error) {
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, nil, "", fmt.Errorf("get working directory: %w", err)
		}
	}

	branch, err := CurrentBranch(ctx, cwd)
	if err != nil {
		return nil, nil, "", fmt.Errorf("current branch: %w", err)
	}
	if branch == "" {
		return nil, nil, "", fmt.Errorf("--local requires a checked-out branch (HEAD is detached)")
	}

	baseRef := opts.BaseRef
	if baseRef == "" {
		baseRef, err = DefaultBranch(ctx, cwd)
		if err != nil {
			return nil, nil, "", err
		}
	}
	if baseRef == branch {
		return nil, nil, "", fmt.Errorf("--local: current branch (%s) is the same as base ref — switch branches or pass a different --base", branch)
	}

	_, owner, repoName, err := OriginRemote(ctx, cwd)
	if err != nil {
		return nil, nil, "", fmt.Errorf("origin remote: %w", err)
	}
	repoFull := owner + "/" + repoName

	headSHA, err := revParse(ctx, cwd, "HEAD")
	if err != nil {
		return nil, nil, "", fmt.Errorf("get head SHA: %w", err)
	}
	baseSHA := getBaseSHA(ctx, cwd, baseRef)
	if baseSHA == "" {
		return nil, nil, "", fmt.Errorf("couldn't resolve base ref %q (tried origin/%s and %s)", baseRef, baseRef, baseRef)
	}

	key := LocalSessionKey(branch, baseRef)
	session, err := LoadSession(owner, repoName, key)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load session: %w", err)
	}

	if session != nil {
		if session.HeadSHA == headSHA && session.BaseSHA == baseSHA {
			pd, pdErr := loadLatestSnapshotDiffLocal(session, owner, repoName, key)
			if pdErr != nil {
				return nil, nil, "", pdErr
			}
			return session, pd, cwd, nil
		}
		return captureNewSnapshotLocal(ctx, session, cwd, branch, baseRef, key, owner, repoName, baseSHA, headSHA)
	}

	now := time.Now().UTC()
	session = &Session{
		ID:         newUUID(),
		Repo:       repoFull,
		Kind:       SessionKindLocal,
		BaseBranch: baseRef,
		BaseSHA:    baseSHA,
		HeadSHA:    headSHA,
		Branch:     branch,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	pd, err := captureLocalSnapshot(ctx, session, cwd, branch, key, owner, repoName, baseSHA, headSHA)
	if err != nil {
		return nil, nil, "", err
	}
	if err := SaveSession(owner, repoName, key, session); err != nil {
		return nil, nil, "", fmt.Errorf("save session: %w", err)
	}
	if err := upsertIndex(owner, repoName, session); err != nil {
		return nil, nil, "", fmt.Errorf("upsert index: %w", err)
	}
	return session, pd, cwd, nil
}

func captureNewSnapshotLocal(ctx context.Context, session *Session, cwd, branch, baseRef string, key SessionKey, org, repoName, baseSHA, headSHA string) (*Session, *ParsedDiff, string, error) {
	pd, err := captureLocalSnapshot(ctx, session, cwd, branch, key, org, repoName, baseSHA, headSHA)
	if err != nil {
		return nil, nil, "", err
	}
	session.BaseSHA = baseSHA
	session.HeadSHA = headSHA
	session.UpdatedAt = time.Now().UTC()
	if err := SaveSession(org, repoName, key, session); err != nil {
		return nil, nil, "", fmt.Errorf("save session: %w", err)
	}
	if err := upsertIndex(org, repoName, session); err != nil {
		return nil, nil, "", fmt.Errorf("upsert index: %w", err)
	}
	return session, pd, cwd, nil
}

func captureLocalSnapshot(ctx context.Context, session *Session, cwd, branch string, key SessionKey, org, repoName, baseSHA, headSHA string) (*ParsedDiff, error) {
	diffBytes, err := gitDiff(ctx, cwd, baseSHA, headSHA)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return writeSnapshot(ctx, session, cwd, branch, key, org, repoName, baseSHA, headSHA, diffBytes)
}

// writeSnapshot persists diff.patch + meta.yaml under the snapshot dir, appends
// the snapshot to session.Snapshots, and returns the parsed diff. Shared by PR
// and local capture paths.
func writeSnapshot(ctx context.Context, session *Session, cwd, branch string, key SessionKey, org, repoName, baseSHA, headSHA string, diffBytes []byte) (*ParsedDiff, error) {
	snapDir := SnapshotDir(org, repoName, key, baseSHA, headSHA)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(snapDir, "diff.patch"), diffBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write diff.patch: %w", err)
	}

	meta := snapshotMeta{
		ObservedAt:        time.Now().UTC(),
		Branch:            branch,
		HeadCommitMessage: headCommitMessage(ctx, cwd, headSHA),
	}
	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal meta.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "meta.yaml"), metaBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write meta.yaml: %w", err)
	}

	snap := Snapshot{Base: baseSHA, Head: headSHA, ObservedAt: meta.ObservedAt}
	session.Snapshots = append(session.Snapshots, snap)

	return Parse(diffBytes)
}

// gitDiff produces a unified diff between base and head using the same
// three-dot semantics GitHub uses for PRs (diff vs the merge-base).
func gitDiff(ctx context.Context, cwd, base, head string) ([]byte, error) {
	args := []string{"diff", base + "..." + head}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...%s: %w", base, head, err)
	}
	return out, nil
}

func loadLatestSnapshotDiff(session *Session, org, repoName string) (*ParsedDiff, error) {
	return loadLatestSnapshotDiffLocal(session, org, repoName, KeyForSession(session))
}

func loadLatestSnapshotDiffLocal(session *Session, org, repoName string, key SessionKey) (*ParsedDiff, error) {
	if len(session.Snapshots) == 0 {
		return &ParsedDiff{}, nil
	}
	snap := session.Snapshots[len(session.Snapshots)-1]
	snapDir := SnapshotDir(org, repoName, key, snap.Base, snap.Head)
	diffBytes, err := os.ReadFile(filepath.Join(snapDir, "diff.patch"))
	if err != nil {
		if os.IsNotExist(err) {
			return &ParsedDiff{}, nil
		}
		return nil, fmt.Errorf("read diff.patch: %w", err)
	}
	return Parse(diffBytes)
}

func loadSessionByID(ctx context.Context, opts ResolveOpts) (*Session, *ParsedDiff, string, error) {
	id := opts.SessionID
	base := BasePath()
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil, "", fmt.Errorf("session %s not found", id)
	}

	var found *Session
	var foundOrg, foundRepo string
	var foundKey SessionKey

	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Base(path) != "session.yaml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var s Session
		if unmarshalErr := yaml.Unmarshal(data, &s); unmarshalErr != nil {
			return nil
		}
		if s.ID == id {
			found = &s
			rel, _ := filepath.Rel(base, path)
			parts := strings.SplitN(filepath.ToSlash(rel), "/", 4)
			if len(parts) >= 3 {
				foundOrg = parts[0]
				foundRepo = parts[1]
				foundKey = SessionKey(parts[2])
			}
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, "", fmt.Errorf("searching sessions: %w", err)
	}
	if found == nil {
		return nil, nil, "", fmt.Errorf("session %s not found", id)
	}

	pd, err := loadLatestSnapshotDiffLocal(found, foundOrg, foundRepo, foundKey)
	if err != nil {
		return nil, nil, "", err
	}

	workspaceDir := findWorktreeForBranch(ctx, opts.CWD, found.Branch)
	return found, pd, workspaceDir, nil
}

// findWorktreeForBranch returns the absolute path of the worktree whose branch
// matches the given branch name, or "" when no match exists.
func findWorktreeForBranch(ctx context.Context, cwd, branch string) string {
	if branch == "" {
		return ""
	}
	wts, err := listWorktrees(ctx, cwd)
	if err != nil {
		return ""
	}
	for _, wt := range wts {
		if wt.Branch == branch {
			return wt.Path
		}
	}
	return ""
}

func upsertIndex(org, repoName string, session *Session) error {
	entries, err := LoadIndex(org, repoName)
	if err != nil {
		return err
	}
	key := KeyForSession(session)
	sessionPath := SessionDir(org, repoName, key)
	for i, e := range entries {
		if e.SessionKey() == key {
			entries[i].Key = string(key)
			entries[i].UpdatedAt = session.UpdatedAt
			entries[i].Path = sessionPath
			return SaveIndex(org, repoName, entries)
		}
	}
	entries = append(entries, SessionsIndexEntry{
		Key:       string(key),
		PRNumber:  session.PRNumber,
		Path:      sessionPath,
		UpdatedAt: session.UpdatedAt,
	})
	return SaveIndex(org, repoName, entries)
}

func revParse(ctx context.Context, cwd, ref string) (string, error) {
	args := []string{"rev-parse", ref}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getBaseSHA(ctx context.Context, worktreePath, baseRefName string) string {
	for _, ref := range []string{"origin/" + baseRefName, baseRefName} {
		sha, err := revParse(ctx, worktreePath, ref)
		if err == nil && sha != "" {
			return sha
		}
	}
	return ""
}

func headCommitMessage(ctx context.Context, cwd, sha string) string {
	args := []string{"log", "-1", "--pretty=%B", sha}
	if cwd != "" {
		args = append([]string{"-C", cwd}, args...)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
