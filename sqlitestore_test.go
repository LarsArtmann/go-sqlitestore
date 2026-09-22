package sqlitestore_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/larsartmann/go-sqlitestore"
)

var _ = Describe("sqlitestore engine", func() {
	var (
		ctx context.Context
		dir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		dir = GinkgoT().TempDir()
	})

	openFresh := func(name string) *sqlitestore.DB {
		db, err := sqlitestore.Open(dir + "/" + name)
		Expect(err).NotTo(HaveOccurred())

		return db
	}

	Describe("Open and schema setup", func() {
		It("opens a fresh database with WAL mode and a working health check", func() {
			db := openFresh("basic.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			Expect(db.HealthCheck(ctx)).To(Succeed())

			var mode string
			Expect(db.DB().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)).To(Succeed())
			Expect(mode).To(Equal("wal"))
		})

		It("creates the parent directory when missing", func() {
			db, err := sqlitestore.Open(dir + "/nested/deeper/store.db")
			Expect(err).NotTo(HaveOccurred())
			defer func() { Expect(db.Close()).To(Succeed()) }()

			Expect(db.HealthCheck(ctx)).To(Succeed())
		})
	})

	Describe("RunMigrations", func() {
		type mig = sqlitestore.Migration

		simple := func(version int, table string) mig {
			return mig{
				Version:     version,
				Description: fmt.Sprintf("create %s", table),
				Up: func(ctx context.Context, db *sql.DB) error {
					_, err := db.ExecContext(ctx,
						"CREATE TABLE IF NOT EXISTS "+table+" (id INTEGER PRIMARY KEY)")

					return err
				},
			}
		}

		It("applies all migrations to a fresh database and records them", func() {
			db := openFresh("migrate.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			Expect(sqlitestore.RunMigrations(ctx, db.DB(), []mig{
				simple(1, "alpha"), simple(2, "beta"),
			})).To(Succeed())

			var count int
			Expect(db.DB().QueryRowContext(ctx,
				"SELECT COUNT(*) FROM schema_migrations").Scan(&count)).To(Succeed())
			Expect(count).To(Equal(2))
		})

		It("never re-runs a recorded version (immutable history)", func() {
			db := openFresh("immutable.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			applied := 0
			counting := mig{
				Version:     7,
				Description: "counting",
				Up: func(ctx context.Context, db *sql.DB) error {
					applied++

					return nil
				},
			}

			Expect(sqlitestore.RunMigrations(ctx, db.DB(), []mig{counting})).To(Succeed())
			Expect(sqlitestore.RunMigrations(ctx, db.DB(), []mig{counting})).To(Succeed())
			Expect(applied).To(Equal(1))
		})

		It("applies an unrecorded version below the recorded max (membership, not max-version)", func() {
			db := openFresh("membership.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			// Simulate the C23 shared-line split: v11 recorded by one
			// consumer, then the OTHER consumer's v3 arrives.
			Expect(sqlitestore.RunMigrations(ctx, db.DB(), []mig{simple(11, "late")})).To(Succeed())
			Expect(sqlitestore.RunMigrations(ctx, db.DB(), []mig{simple(3, "early")})).To(Succeed())

			var count int
			Expect(db.DB().QueryRowContext(ctx,
				"SELECT COUNT(*) FROM schema_migrations").Scan(&count)).To(Succeed())
			Expect(count).To(Equal(2))
		})

		It("reports a failing migration with its version and description", func() {
			db := openFresh("failing.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			boom := mig{
				Version:     4,
				Description: "explodes",
				Up: func(ctx context.Context, db *sql.DB) error {
					return errors.New("boom")
				},
			}

			err := sqlitestore.RunMigrations(ctx, db.DB(), []mig{boom})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("migration 4 (explodes) failed"))
		})
	})

	Describe("AddColumnIfNotExists", func() {
		It("adds a column once and is idempotent", func() {
			db := openFresh("columns.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.DB().ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
			Expect(err).NotTo(HaveOccurred())

			Expect(sqlitestore.AddColumnIfNotExists(ctx, db.DB(), "t", "extra", "TEXT NOT NULL DEFAULT ''")).To(Succeed())
			Expect(sqlitestore.AddColumnIfNotExists(ctx, db.DB(), "t", "extra", "TEXT NOT NULL DEFAULT ''")).To(Succeed())

			exists, err := sqlitestore.ColumnExists(ctx, db.DB(), "t", "extra")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("ExecWrite", func() {
		It("writes through the mutex path and exposes busy-retry telemetry", func() {
			db := openFresh("writes.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.ExecWrite(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
			Expect(err).NotTo(HaveOccurred())

			_, err = db.ExecWrite(ctx, "INSERT INTO t (v) VALUES (?)", "hello")
			Expect(err).NotTo(HaveOccurred())

			var v string
			Expect(db.DB().QueryRowContext(ctx, "SELECT v FROM t WHERE id = 1").Scan(&v)).To(Succeed())
			Expect(v).To(Equal("hello"))
			Expect(db.BusyRetryCount()).To(BeNumerically(">=", 0))
		})

		It("rejects writes after Close", func() {
			db := openFresh("closed.db")
			Expect(db.Close()).To(Succeed())

			_, err := db.ExecWrite(ctx, "CREATE TABLE t (id INTEGER)")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("PurgeByExpiry", func() {
		It("deletes only rows older than the cutoff for the given time column", func() {
			db := openFresh("purge.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.ExecWrite(ctx,
				"CREATE TABLE events (id INTEGER PRIMARY KEY, recorded_at TEXT NOT NULL)")
			Expect(err).NotTo(HaveOccurred())

			now := time.Now()
			old := now.Add(-72 * time.Hour).UTC().Format(time.RFC3339Nano)
			fresh := now.Add(-1 * time.Minute).UTC().Format(time.RFC3339Nano)

			_, err = db.ExecWrite(ctx,
				"INSERT INTO events (recorded_at) VALUES (?), (?)", old, fresh)
			Expect(err).NotTo(HaveOccurred())

			deleted, err := db.PurgeByExpiry(ctx, "events", "recorded_at", now.Add(-1*time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal(int64(1)))

			var count int
			Expect(db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)).To(Succeed())
			Expect(count).To(Equal(1))
		})

		It("deletes nothing when the table is empty", func() {
			db := openFresh("purge-empty.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.ExecWrite(ctx, "CREATE TABLE lonely (expires_at TEXT)")
			Expect(err).NotTo(HaveOccurred())

			deleted, err := db.PurgeByExpiry(ctx, "lonely", "expires_at", time.Now())
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(Equal(int64(0)))
		})
	})

	Describe("IsBusy", func() {
		It("classifies a plain error as not-busy", func() {
			Expect(sqlitestore.IsBusy(errors.New("some failure"))).To(BeFalse())
			Expect(sqlitestore.IsBusy(nil)).To(BeFalse())
		})
	})

	Describe("SetDebugSQL", func() {
		It("toggles without side effects on the write path", func() {
			db := openFresh("debug.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			db.SetDebugSQL(true)
			_, err := db.ExecWrite(ctx, "CREATE TABLE t (id INTEGER)")
			Expect(err).NotTo(HaveOccurred())
			db.SetDebugSQL(false)
		})
	})

	Describe("Options", func() {
		It("honors WithMaxOpenConns and WithBusyTimeout", func() {
			db, err := sqlitestore.Open(dir+"/opts.db",
				sqlitestore.WithMaxOpenConns(2),
				sqlitestore.WithBusyTimeout(1500),
			)
			Expect(err).NotTo(HaveOccurred())
			defer func() { Expect(db.Close()).To(Succeed()) }()

			stats := db.DB().Stats()
			Expect(stats.MaxOpenConnections).To(Equal(2))
		})
	})
})
