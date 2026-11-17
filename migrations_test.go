package sqlitestore_test

import (
	"context"
	"database/sql"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/larsartmann/go-sqlitestore"
)

var _ = Describe("sqlitestore migrations", func() {
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

	Describe("RunMigrations", func() {
		type mig = sqlitestore.Migration

		simple := func(version int, table string) mig {
			return mig{
				Version:     version,
				Description: "create " + table,
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

			Expect(
				sqlitestore.AddColumnIfNotExists(ctx, db.DB(), "t", "extra", "TEXT NOT NULL DEFAULT ''"),
			).To(Succeed())
			Expect(
				sqlitestore.AddColumnIfNotExists(ctx, db.DB(), "t", "extra", "TEXT NOT NULL DEFAULT ''"),
			).To(Succeed())

			exists, err := sqlitestore.ColumnExists(ctx, db.DB(), "t", "extra")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("TableExists", func() {
		It("reports table existence accurately", func() {
			db := openFresh("tables.db")
			defer func() { Expect(db.Close()).To(Succeed()) }()

			_, err := db.DB().ExecContext(ctx, "CREATE TABLE real (id INTEGER PRIMARY KEY)")
			Expect(err).NotTo(HaveOccurred())

			exists, err := sqlitestore.TableExists(ctx, db.DB(), "real")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())

			exists, err = sqlitestore.TableExists(ctx, db.DB(), "missing")
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Describe("IsDuplicateColumnError", func() {
		It("matches only the duplicate-column case", func() {
			Expect(sqlitestore.IsDuplicateColumnError(
				errors.New("duplicate column name: extra"))).To(BeTrue())
			Expect(sqlitestore.IsDuplicateColumnError(
				errors.New("no such table: t"))).To(BeFalse())
			Expect(sqlitestore.IsDuplicateColumnError(nil)).To(BeFalse())
		})
	})
})
