package sqlitestore_test

import (
	"context"
	"errors"
	"strings"
	"sync"
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

	Describe("WriteLock primitives", func() {
		It("serializes multi-statement units through WithWriteLock", func() {
			db := openFresh("write-lock.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.ExecWrite(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
			Expect(err).NotTo(HaveOccurred())

			const goroutines = 4

			var (
				wg       sync.WaitGroup
				sequence []int
			)

			for i := range goroutines {
				wg.Add(1)

				go func(slot int) {
					defer GinkgoRecover()
					defer wg.Done()

					Expect(db.WithWriteLock(func() error {
						_, err := db.DB().ExecContext(ctx,
							"INSERT INTO t (id) VALUES (?)", slot)
						if err != nil {
							return err
						}

						sequence = append(sequence, slot)

						return nil
					})).To(Succeed())
				}(i + 1)
			}

			wg.Wait()
			Expect(sequence).To(HaveLen(goroutines))

			var count int
			Expect(db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM t").Scan(&count)).To(Succeed())
			Expect(count).To(Equal(goroutines))
		})

		It("exposes WriteLock with an explicit release", func() {
			db := openFresh("write-lock-explicit.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			release := db.WriteLock()
			_, err := db.DB().ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
			Expect(err).NotTo(HaveOccurred())
			release()

			// After release, ExecWrite must be able to take the mutex again.
			_, err = db.ExecWrite(ctx, "INSERT INTO t (id) VALUES (1)")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("IsBusy", func() {
		It("classifies a plain error as not-busy", func() {
			Expect(sqlitestore.IsBusy(errors.New("some failure"))).To(BeFalse())
			Expect(sqlitestore.IsBusy(nil)).To(BeFalse())
		})

		It("classifies a real SQLITE_BUSY from a contending writer", func() {
			path := dir + "/contended.db"

			blocker, err := sqlitestore.OpenDB(path, sqlitestore.WithBusyTimeout(0))

			Expect(err).NotTo(HaveOccurred())
			defer func() { Expect(blocker.Close()).To(Succeed()) }()

			contender, err := sqlitestore.OpenDB(path, sqlitestore.WithBusyTimeout(0))

			Expect(err).NotTo(HaveOccurred())
			defer func() { Expect(contender.Close()).To(Succeed()) }()

			_, err = blocker.ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY)")
			Expect(err).NotTo(HaveOccurred())

			tx, err := blocker.BeginTx(ctx, nil)

			Expect(err).NotTo(HaveOccurred())
			defer func() { Expect(tx.Rollback()).To(Succeed()) }()

			_, err = tx.ExecContext(ctx, "INSERT INTO t (id) VALUES (1)")
			Expect(err).NotTo(HaveOccurred())

			_, err = contender.ExecContext(ctx, "INSERT INTO t (id) VALUES (2)")
			Expect(err).To(HaveOccurred())
			Expect(sqlitestore.IsBusy(err)).To(BeTrue(),
				"expected SQLITE_BUSY from the contending writer, got: %v", err)
		})
	})

	Describe("TruncateSQL", func() {
		It("keeps short queries intact and truncates long ones", func() {
			short := "SELECT 1"
			Expect(sqlitestore.TruncateSQL(short)).To(Equal(short))

			long := strings.Repeat("x", 250)
			truncated := sqlitestore.TruncateSQL(long)
			Expect(truncated).To(HaveLen(203))
			Expect(truncated).To(HaveSuffix("..."))
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
