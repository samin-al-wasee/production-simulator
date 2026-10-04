package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func twoUsers(t *testing.T, st *Store) (User, User) {
	t.Helper()
	ctx := context.Background()
	a, err := st.UpsertUserByOAuth(ctx, "github", "own-a-"+unique("x"), unique("a")+"@example.com", "A", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.UpsertUserByOAuth(ctx, "github", "own-b-"+unique("y"), unique("b")+"@example.com", "B", "")
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestSandboxCRUDAndOwnership(t *testing.T) {
	st := integrationStore(t)
	ctx := context.Background()
	owner, other := twoUsers(t, st)

	sum, err := st.CreateSandbox(ctx, owner.ID, "first", "sandbox/v14", 7, 12, `{"ruleset":"sandbox/v14","seed":7,"tick":12,"log":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if sum.ID == "" || sum.Name != "first" || sum.Tick != 12 {
		t.Fatalf("summary = %+v", sum)
	}

	got, err := st.GetSandbox(ctx, owner.ID, sum.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "first" || got.Save == "" {
		t.Fatalf("got = %+v", got)
	}

	// The sandbox belongs to its owner alone.
	if _, err := st.GetSandbox(ctx, other.ID, sum.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user get error = %v", err)
	}
	if err := st.UpdateSandbox(ctx, other.ID, sum.ID, "hijack", 99, "{}"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user update error = %v", err)
	}
	if err := st.DeleteSandbox(ctx, other.ID, sum.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user delete error = %v", err)
	}

	if err := st.UpdateSandbox(ctx, owner.ID, sum.ID, "renamed", 30, `{"ruleset":"sandbox/v14","seed":7,"tick":30,"log":[]}`); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListSandboxes(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range list {
		if s.ID == sum.ID {
			found = true
			if s.Name != "renamed" || s.Tick != 30 {
				t.Fatalf("updated summary = %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("sandbox missing from the owner's list")
	}
	if otherList, err := st.ListSandboxes(ctx, other.ID); err != nil || len(otherList) != 0 {
		t.Fatalf("other user's list = %+v, err = %v", otherList, err)
	}

	if err := st.DeleteSandbox(ctx, owner.ID, sum.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSandbox(ctx, owner.ID, sum.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete error = %v", err)
	}
}

func TestRecordProgressIsPerUserAndFirstWins(t *testing.T) {
	st := integrationStore(t)
	ctx := context.Background()
	a, b := twoUsers(t, st)
	now := time.Now()

	done, err := st.RecordProgress(ctx, a.ID, []string{"s1-a", "s1-b"}, "manual", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 2 {
		t.Fatalf("first record = %v", done)
	}
	again, err := st.RecordProgress(ctx, a.ID, []string{"s1-a"}, "sandbox game-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("re-recording added %v; the first record must win", again)
	}

	entries, err := st.ProgressForUser(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		if e.CompletedBy != "manual" {
			t.Fatalf("completed_by = %q", e.CompletedBy)
		}
	}

	if other, err := st.ProgressForUser(ctx, b.ID); err != nil || len(other) != 0 {
		t.Fatalf("other user's progress = %+v, err = %v", other, err)
	}
}
