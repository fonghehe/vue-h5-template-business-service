package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

type stickyProductCache struct {
	recordingProductCache
}

func (*stickyProductCache) Delete(context.Context, uint) error {
	return errors.New("cache unavailable")
}

func cachedProductFixture(t *testing.T) *ProductService {
	t.Helper()
	fixture := newCommerceFixture(t)
	products := NewProductService(repository.NewProductRepository(fixture.db), fixture.repo, &stickyProductCache{}, nil)
	if _, err := products.Detail(context.Background(), 1); err != nil {
		t.Fatalf("Detail(1) initial cache fill failed: %v", err)
	}
	return products
}

func checkProductNotFound(t *testing.T, products *ProductService) {
	t.Helper()
	_, err := products.Detail(context.Background(), 1)
	var appError *apierr.Error
	if !errors.As(err, &appError) || appError.Code != apierr.CodeNotFound {
		t.Errorf("Detail(1) with stale cache error = %v, want code %d", err, apierr.CodeNotFound)
	}
}

func TestCachedProductDetailRejectsUnlistedProducts(t *testing.T) {
	for _, status := range []string{model.StatusDraft, model.StatusSoldOut} {
		t.Run(status, func(t *testing.T) {
			products := cachedProductFixture(t)
			if _, err := products.Patch(context.Background(), 1, ProductPatchInput{Status: &status}); err != nil {
				t.Fatalf("Patch(1, status=%q) error = %v, want nil", status, err)
			}
			checkProductNotFound(t, products)
		})
	}
}

func TestCachedProductDetailRejectsDeletedProducts(t *testing.T) {
	products := cachedProductFixture(t)
	if err := products.Delete(context.Background(), 1); err != nil {
		t.Fatalf("Delete(1) error = %v, want nil", err)
	}
	checkProductNotFound(t, products)
}

func TestFavoriteStatusAvoidsCatalogueAssociationReads(t *testing.T) {
	fixture := newCommerceFixture(t)
	var queries atomic.Int64
	if err := fixture.db.Callback().Query().Before("gorm:query").Register("count_status_queries", func(*gorm.DB) {
		queries.Add(1)
	}); err != nil {
		t.Fatalf("register query counter: %v", err)
	}
	status, err := fixture.services.Favorites.Status(context.Background(), fixture.userID, 1)
	if err != nil {
		t.Fatalf("Favorites.Status(user=%d, product=1) error = %v, want nil", fixture.userID, err)
	}
	if status.ProductID != 1 || status.Favorite {
		t.Errorf("Favorites.Status(user=%d, product=1) = %+v, want product=1, favorite=false", fixture.userID, status)
	}
	if got := queries.Load(); got != 2 {
		t.Errorf("Favorites.Status(user=%d, product=1) queries = %d, want 2 without SKU preloads", fixture.userID, got)
	}
}

func TestFavoriteSetAvoidsCatalogueAssociationReads(t *testing.T) {
	fixture := newCommerceFixture(t)
	var queries atomic.Int64
	if err := fixture.db.Callback().Query().Before("gorm:query").Register("count_set_queries", func(*gorm.DB) {
		queries.Add(1)
	}); err != nil {
		t.Fatalf("register query counter: %v", err)
	}
	status, err := fixture.services.Favorites.Set(context.Background(), fixture.userID, 1, true)
	if err != nil {
		t.Fatalf("Favorites.Set(user=%d, product=1, true) error = %v, want nil", fixture.userID, err)
	}
	if status.ProductID != 1 || !status.Favorite {
		t.Errorf("Favorites.Set(user=%d, product=1, true) = %+v, want product=1, favorite=true", fixture.userID, status)
	}
	if got := queries.Load(); got != 1 {
		t.Errorf("Favorites.Set(user=%d, product=1, true) queries = %d, want 1 without SKU preloads", fixture.userID, got)
	}
}

func TestCachedProductDetailPreservesCancellation(t *testing.T) {
	products := cachedProductFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := products.Detail(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("Detail(1) with cancelled context error = %v, want context.Canceled", err)
	}
}
