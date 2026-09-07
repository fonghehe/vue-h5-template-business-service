package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestInventoryConcurrencyPreventsOverselling(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=30000", filepath.Join(t.TempDir(), "inventory.db"))
	db, err := database.Open(database.Options{
		Driver: "sqlite", DSN: dsn, MaxOpenConns: 20, MaxIdleConns: 20, AutoMigrate: true, Quiet: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Create(&model.Inventory{SKUID: 99, Available: 10, Reserved: 0, Version: 1}).Error)
	repo := NewCommerceRepository(db)

	var succeeded atomic.Int64
	var insufficient atomic.Int64
	var unexpected atomic.Int64
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			err := repo.ReserveInventory(context.Background(), 99, 1)
			switch {
			case err == nil:
				succeeded.Add(1)
			case IsConflict(err):
				insufficient.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()

	assert.Equal(t, int64(10), succeeded.Load())
	assert.Equal(t, int64(90), insufficient.Load())
	assert.Zero(t, unexpected.Load())
	inventory, err := repo.Inventory(context.Background(), 99)
	require.NoError(t, err)
	assert.Equal(t, int64(0), inventory.Available)
	assert.Equal(t, int64(10), inventory.Reserved)
	assert.GreaterOrEqual(t, inventory.Available, int64(0))

	require.NoError(t, repo.ReleaseInventory(context.Background(), 99, 10))
	inventory, err = repo.Inventory(context.Background(), 99)
	require.NoError(t, err)
	assert.Equal(t, int64(10), inventory.Available)
	assert.Equal(t, int64(0), inventory.Reserved)
}
