package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("FORGELAB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FORGELAB_TEST_DATABASE_URL to run store integration tests")
	}
	ctx := context.Background()
	st, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

func unique(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestUpsertUserByOAuth(t *testing.T) {
	st := integrationStore(t)
	ctx := context.Background()
	suffix := unique("itest")
	email := suffix + "@example.com"

	first, err := st.UpsertUserByOAuth(ctx, "github", "gh-"+suffix, email, "Ada", "http://avatar/1")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.DisplayName != "Ada" {
		t.Fatalf("first = %+v", first)
	}

	// The same provider identity resolves to the same user and refreshes the profile.
	second, err := st.UpsertUserByOAuth(ctx, "github", "gh-"+suffix, email, "Ada Lovelace", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("re-sign-in created a new user: %s vs %s", second.ID, first.ID)
	}
	if second.DisplayName != "Ada Lovelace" || second.AvatarURL != "http://avatar/1" {
		t.Fatalf("profile not refreshed with empty fields preserved: %+v", second)
	}

	// The same email under another provider links to the same user.
	linked, err := st.UpsertUserByOAuth(ctx, "google", "go-"+suffix, email, "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if linked.ID != first.ID {
		t.Fatalf("email match did not link: %s vs %s", linked.ID, first.ID)
	}

	// A different email is a different user.
	other, err := st.UpsertUserByOAuth(ctx, "github", "gh2-"+suffix, "other-"+suffix+"@example.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == first.ID {
		t.Fatal("distinct emails produced the same user")
	}
}

func TestSessionLifecycle(t *testing.T) {
	st := integrationStore(t)
	ctx := context.Background()
	suffix := unique("itest")
	u, err := st.UpsertUserByOAuth(ctx, "github", "gh-"+suffix, suffix+"@example.com", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}

	live := []byte("live-" + suffix)
	if err := st.CreateSession(ctx, live, u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := st.SessionUser(ctx, live)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || got.Email != u.Email {
		t.Fatalf("session user = %+v, want %+v", got, u)
	}

	expired := []byte("expired-" + suffix)
	if err := st.CreateSession(ctx, expired, u.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SessionUser(ctx, expired); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session error = %v", err)
	}

	if err := st.DeleteSession(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SessionUser(ctx, live); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session error = %v", err)
	}
	if err := st.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
}
