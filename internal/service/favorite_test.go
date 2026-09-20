package service

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestFavoritesPaginateVisibleRowsWithBoundedQueries(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	for _, id := range []uint{1, 2, 3} {
		require.NoError(t, fixture.db.Create(&model.Favorite{UserID: fixture.userID, ProductID: id}).Error)
	}
	require.NoError(t, fixture.db.Delete(&model.Product{}, 2).Error)
	var queries atomic.Int64
	require.NoError(t, fixture.db.Callback().Query().Before("gorm:query").Register("count_favorite_queries", func(*gorm.DB) {
		queries.Add(1)
	}))
	page, err := fixture.services.Favorites.List(ctx, fixture.userID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, uint(3), page.Items[0].ID)
	assert.Equal(t, uint(1), page.Items[1].ID)
	assert.False(t, page.HasMore)
	assert.LessOrEqual(t, queries.Load(), int64(4), "page query count must not grow per product")
	page, err = fixture.services.Favorites.List(ctx, fixture.userID, 3, 2)
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.Equal(t, int64(2), page.Total)
}
