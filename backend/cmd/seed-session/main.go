// Command seed-session creates a user and a database-backed session, then
// prints the raw session token. It exists for the browser tests, which cannot
// run a real OAuth round trip; it is NOT part of the deployed server (the image
// builds only ./cmd/forgelab) and refuses to run without FORGELAB_TEST_SESSION=1.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/store"
)

func main() {
	if os.Getenv("FORGELAB_TEST_SESSION") != "1" {
		fmt.Fprintln(os.Stderr, "seed-session: set FORGELAB_TEST_SESSION=1 to confirm this is a test run")
		os.Exit(2)
	}
	dsn := flag.String("database", os.Getenv("DATABASE_URL"), "Postgres URL")
	email := flag.String("email", "e2e@forgelab.test", "email for the seeded user")
	name := flag.String("name", "E2E Player", "display name for the seeded user")
	flag.Parse()
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "seed-session: -database or DATABASE_URL is required")
		os.Exit(2)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, *dsn)
	if err != nil {
		fail(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		fail(err)
	}

	user, err := st.UpsertUserByOAuth(ctx, "test", "e2e-"+*email, *email, *name, "")
	if err != nil {
		fail(err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fail(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	if err := st.CreateSession(ctx, hash[:], user.ID, time.Now().Add(24*time.Hour)); err != nil {
		fail(err)
	}
	fmt.Println(token)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "seed-session: %v\n", err)
	os.Exit(1)
}
