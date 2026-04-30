package chathistory

import (
	"path/filepath"
	"sync"
	"testing"
	"unicode/utf8"
)

func newTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store := New(path)
	if err := store.Err(); err != nil {
		t.Fatalf("store unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store failed: %v", err)
		}
	})
	return store
}

func TestStoreCreatesAndPersistsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)

	started, err := store.Start(StartParams{
		CallerID:  "caller:abc",
		AccountID: "user@example.com",
		Model:     "deepseek-v4-flash",
		Stream:    true,
		UserInput: "hello",
		Messages:  []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("start entry failed: %v", err)
	}

	updated, err := store.Update(started.ID, UpdateParams{
		Status:           "success",
		ReasoningContent: "thinking",
		Content:          "answer",
		StatusCode:       200,
		ElapsedMs:        321,
		FinishReason:     "stop",
		Usage:            map[string]any{"total_tokens": 9},
		Completed:        true,
	})
	if err != nil {
		t.Fatalf("update entry failed: %v", err)
	}
	if updated.Status != "success" || updated.Content != "answer" {
		t.Fatalf("unexpected updated entry: %#v", updated)
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshot.Limit != DefaultLimit {
		t.Fatalf("unexpected default limit: %d", snapshot.Limit)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(snapshot.Items))
	}
	if snapshot.Items[0].CompletedAt == 0 {
		t.Fatalf("expected completed_at to be populated")
	}
	if snapshot.Items[0].Preview != "answer" {
		t.Fatalf("expected summary preview=answer, got %#v", snapshot.Items[0])
	}
	if snapshot.Stats.TotalCalls != 0 {
		t.Fatalf("expected default stats to be zero, got %#v", snapshot.Stats)
	}

	reloaded := newTestStore(t, path)
	reloadedSnapshot, err := reloaded.Snapshot()
	if err != nil {
		t.Fatalf("reload snapshot failed: %v", err)
	}
	if len(reloadedSnapshot.Items) != 1 {
		t.Fatalf("unexpected reloaded summaries: %#v", reloadedSnapshot.Items)
	}
	full, err := reloaded.Get(started.ID)
	if err != nil {
		t.Fatalf("get detail failed: %v", err)
	}
	if full.Content != "answer" {
		t.Fatalf("expected detail content=answer, got %#v", full)
	}
	if len(full.Messages) != 1 || full.Messages[0].Content != "hello" {
		t.Fatalf("expected messages persisted, got %#v", full.Messages)
	}
}

func TestBuildPreviewPreservesUTF8MB4Characters(t *testing.T) {
	long := strings.Repeat("😀", defaultPreviewAt+1)
	preview := buildPreview(Entry{Content: long})
	if !utf8.ValidString(preview) {
		t.Fatalf("expected valid utf-8 preview, got %q", preview)
	}
	if preview != strings.Repeat("😀", defaultPreviewAt)+"..." {
		t.Fatalf("unexpected preview: %q", preview)
	}
}

func TestStoreTrimsToConfiguredLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)
	if _, err := store.SetLimit(10); err != nil {
		t.Fatalf("set limit failed: %v", err)
	}

	for i := 0; i < 12; i++ {
		entry, err := store.Start(StartParams{Model: "deepseek-v4-flash", UserInput: "msg"})
		if err != nil {
			t.Fatalf("start %d failed: %v", i, err)
		}
		if _, err := store.Update(entry.ID, UpdateParams{Status: "success", Content: "ok", Completed: true}); err != nil {
			t.Fatalf("update %d failed: %v", i, err)
		}
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(snapshot.Items))
	}
}

func TestStoreDeleteClearAndLimitValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)
	entry, err := store.Start(StartParams{UserInput: "hello"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := store.Delete(entry.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Items) != 0 {
		t.Fatalf("expected empty items after delete, got %d", len(snapshot.Items))
	}
	if _, err := store.SetLimit(999); err == nil {
		t.Fatalf("expected invalid limit error")
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
}

func TestStoreDisablePreservesHistoryAndBlocksNewEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)

	entry, err := store.Start(StartParams{UserInput: "hello"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if _, err := store.Update(entry.ID, UpdateParams{Status: "success", Content: "world", Completed: true}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	snapshot, err := store.SetLimit(DisabledLimit)
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if snapshot.Limit != DisabledLimit {
		t.Fatalf("expected disabled limit, got %d", snapshot.Limit)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected disabled mode to preserve summaries, got %d", len(snapshot.Items))
	}
	if store.Enabled() {
		t.Fatalf("expected store to report disabled")
	}
	if _, err := store.Start(StartParams{UserInput: "later"}); err != ErrDisabled {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestStoreConcurrentUpdatesKeepRowsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			entry, err := store.Start(StartParams{
				CallerID:  "caller:test",
				Model:     "deepseek-v4-flash",
				UserInput: "hello",
			})
			if err != nil {
				t.Errorf("start failed: %v", err)
				return
			}
			_, err = store.Update(entry.ID, UpdateParams{
				Status:    "success",
				Content:   "answer",
				ElapsedMs: int64(idx),
				Completed: true,
			})
			if err != nil {
				t.Errorf("update failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Items) != 8 {
		t.Fatalf("expected 8 items, got %d", len(snapshot.Items))
	}
}

func TestStoreCallStatsPersistAndClearKeepsStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.db")
	store := newTestStore(t, path)

	entry, err := store.Start(StartParams{UserInput: "hello"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if _, err := store.Update(entry.ID, UpdateParams{Status: "success", Content: "ok", Completed: true}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

<<<<<<< HEAD
	codes := []int{200, 302, 400, 500}
	for _, code := range codes {
		if err := store.RecordCall(code); err != nil {
			t.Fatalf("record call failed: %v", err)
		}
=======
	store := New(path)
	if err := store.Err(); err != nil {
		t.Fatalf("expected legacy migration success, got %v", err)
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one migrated summary, got %#v", snapshot.Items)
	}
	full, err := store.Get("chat_legacy")
	if err != nil {
		t.Fatalf("get migrated detail failed: %v", err)
	}
	if full.Content != "world" {
		t.Fatalf("expected migrated detail content preserved, got %#v", full)
	}
}

func TestStoreAutoMigratesMetadataOnlyLegacyMonolith(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.json")
	legacy := legacyFile{
		Version: 1,
		Limit:   20,
		Items: []Entry{{
			ID:           "chat_metadata_only",
			Revision:     0,
			CreatedAt:    1,
			UpdatedAt:    2,
			Status:       "error",
			CallerID:     "caller:test",
			AccountID:    "acct:test",
			Model:        "deepseek-v4-flash",
			Stream:       true,
			UserInput:    "hello",
			Error:        "boom",
			StatusCode:   500,
			ElapsedMs:    12,
			FinishReason: "error",
		}},
	}
	body, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write legacy file failed: %v", err)
	}

	store := New(path)
	if err := store.Err(); err != nil {
		t.Fatalf("expected legacy metadata-only migration success, got %v", err)
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one migrated summary, got %#v", snapshot.Items)
	}
	full, err := store.Get("chat_metadata_only")
	if err != nil {
		t.Fatalf("get migrated detail failed: %v", err)
	}
	if full.Error != "boom" || full.UserInput != "hello" {
		t.Fatalf("expected metadata-only legacy fields preserved, got %#v", full)
	}
	if _, err := os.Stat(filepath.Join(store.DetailDir(), "chat_metadata_only.json")); err != nil {
		t.Fatalf("expected migrated detail file to exist: %v", err)
	}
}

func TestStoreLegacyMigrationBestEffortWhenRewriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat_history.json")
	longID := "chat_" + strings.Repeat("x", 320)
	legacy := legacyFile{
		Version: 1,
		Limit:   20,
		Items: []Entry{{
			ID:        longID,
			CreatedAt: 1,
			UpdatedAt: 2,
			Status:    "success",
			UserInput: "hello",
			Content:   "world",
		}},
	}
	body, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy file failed: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write legacy file failed: %v", err)
	}

	store := New(path)
	if err := store.Err(); err != nil {
		t.Fatalf("expected store to stay usable after migration writeback failure, got %v", err)
	}
	if !store.Enabled() {
		t.Fatal("expected store to remain enabled after best-effort migration")
>>>>>>> upstream/main
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshot.Stats.TotalCalls != 4 || snapshot.Stats.SuccessCalls != 2 || snapshot.Stats.FailedCalls != 2 {
		t.Fatalf("unexpected stats: %#v", snapshot.Stats)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	afterClear, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot after clear failed: %v", err)
	}
	if len(afterClear.Items) != 0 {
		t.Fatalf("expected empty history after clear, got %d", len(afterClear.Items))
	}
	if afterClear.Stats != snapshot.Stats {
		t.Fatalf("expected stats to stay unchanged after clear, before=%#v after=%#v", snapshot.Stats, afterClear.Stats)
	}

	reloaded := newTestStore(t, path)
	reloadedSnapshot, err := reloaded.Snapshot()
	if err != nil {
		t.Fatalf("reload snapshot failed: %v", err)
	}
	if reloadedSnapshot.Stats != snapshot.Stats {
		t.Fatalf("expected stats persisted across restart, got %#v", reloadedSnapshot.Stats)
	}
}
