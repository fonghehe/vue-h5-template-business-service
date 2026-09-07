package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

type recordingProductCache struct {
	product model.Product
	found   bool
	sets    int
	deletes []uint
}

func (c *recordingProductCache) Get(context.Context, uint) (model.Product, bool, error) {
	return c.product, c.found, nil
}
func (c *recordingProductCache) Set(_ context.Context, product model.Product) error {
	c.product, c.found, c.sets = product, true, c.sets+1
	return nil
}
func (c *recordingProductCache) Delete(_ context.Context, id uint) error {
	c.found = false
	c.deletes = append(c.deletes, id)
	return nil
}
func (c *recordingProductCache) Close() error { return nil }

func TestProductDetailUsesCacheAsideAndUpdateInvalidates(t *testing.T) {
	fixture := newCommerceFixture(t)
	cache := &recordingProductCache{}
	products := repository.NewProductRepository(fixture.db)
	service := NewProductService(products, fixture.repo, cache, nil)
	ctx := context.Background()

	first, err := service.Detail(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.sets)
	require.NotEmpty(t, first.SKUs)
	require.NotNil(t, first.SKUs[0].Inventory)
	assert.Nil(t, cache.product.SKUs[0].Inventory, "inventory must not enter the product cache")
	require.NoError(t, fixture.db.Model(&model.Inventory{}).Where("sku_id = ?", first.SKUs[0].ID).
		Update("available", first.SKUs[0].Inventory.Available-1).Error)
	second, err := service.Detail(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, 1, cache.sets, "a cache hit must not query-and-fill again")
	require.NotNil(t, second.SKUs[0].Inventory)
	assert.Equal(t, first.SKUs[0].Inventory.Available-1, second.SKUs[0].Inventory.Available)

	_, err = service.Update(ctx, first.ID, ProductInput{
		Name: first.Name, CategoryID: first.CategoryID, Brand: first.Brand, Cover: first.Cover,
		Title: first.Title, ImgURL: first.ImgURL, Price: first.Price, VipPrice: first.VipPrice,
		ShopDesc: first.ShopDesc, Delivery: first.Delivery, ShopName: first.ShopName,
		Description: first.Description, Stock: first.Stock, Status: first.Status, Featured: first.Featured,
	})
	require.NoError(t, err)
	assert.Contains(t, cache.deletes, first.ID)
}
