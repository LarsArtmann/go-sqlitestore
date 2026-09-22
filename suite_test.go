package sqlitestore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSQLitestore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SQLitestore Suite")
}
