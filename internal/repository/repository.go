// Package repository isolates all SQL from the rest of the service. Handlers
// never touch *gorm.DB directly, which keeps the storage engine swappable and
// makes the service layer testable without a database.
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// UserRepository persists accounts.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository builds a user repository.
func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

// FindByUsername looks up an account by login name.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("username = ?", strings.TrimSpace(username)).
		First(&user).Error
	if err != nil {
		return model.User{}, translate(err, "user not found")
	}
	return user, nil
}

// FindByID looks up an account by primary key.
func (r *UserRepository) FindByID(ctx context.Context, id uint) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return model.User{}, translate(err, "user not found")
	}
	return user, nil
}

// ProductListQuery carries the filters accepted by the public product list.
type ProductListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	// IncludeHidden exposes draft and sold-out items; operator only.
	IncludeHidden bool
	// Status filters by an explicit status; ignored unless IncludeHidden.
	Status   string
	Featured *bool
	Sort     string
	Category string
	PriceMin *int64
	PriceMax *int64
}

// ProductRepository persists catalog items.
type ProductRepository struct {
	db *gorm.DB
}

// NewProductRepository builds a product repository.
func NewProductRepository(db *gorm.DB) *ProductRepository { return &ProductRepository{db: db} }

// List returns one page of products plus the total matching row count.
func (r *ProductRepository) List(ctx context.Context, query ProductListQuery) ([]model.Product, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.Product{})

	// Public callers only ever see items on sale; the operator UI passes
	// IncludeHidden to manage drafts and sold-out stock.
	if !query.IncludeHidden {
		db = db.Where("status = ?", model.StatusOnSale)
	} else if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("LOWER(search_text) LIKE LOWER(?) ESCAPE '\\'", "%"+escapeLike(keyword)+"%")
	}
	if query.Featured != nil {
		db = db.Where("featured = ?", *query.Featured)
	}
	if query.Category != "" {
		db = db.Where("category_id = ?", strings.TrimSpace(query.Category))
	}
	if query.PriceMin != nil || query.PriceMax != nil {
		predicate := "EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id AND ps.status = ?"
		args := []any{model.SKUStatusActive}
		if query.PriceMin != nil {
			predicate += " AND ps.price >= ?"
			args = append(args, *query.PriceMin)
		}
		if query.PriceMax != nil {
			predicate += " AND ps.price <= ?"
			args = append(args, *query.PriceMax)
		}
		db = db.Where(predicate+")", args...)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apierr.Wrap(apierr.CodeInternal, "could not count products", err)
	}

	offset := (query.Page - 1) * query.PageSize
	var products []model.Product
	if err := db.Order(orderFor(query.Sort)).Offset(offset).Limit(query.PageSize).Find(&products).Error; err != nil {
		return nil, 0, apierr.Wrap(apierr.CodeInternal, "could not load products", err)
	}
	return products, total, nil
}

// FindByID returns a single product, including hidden ones.
func (r *ProductRepository) FindByID(ctx context.Context, id uint) (model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).Preload("SKUs").Preload("SKUs.Inventory").First(&product, id).Error; err != nil {
		return model.Product{}, translate(err, "product not found")
	}
	return product, nil
}

// FindPublicByID returns a product only when it is on sale.
func (r *ProductRepository) FindPublicByID(ctx context.Context, id uint) (model.Product, error) {
	var product model.Product
	if err := r.db.WithContext(ctx).Preload("SKUs").Where("status = ?", model.StatusOnSale).First(&product, id).Error; err != nil {
		return model.Product{}, translate(err, "product not found")
	}
	return product, nil
}

// AttachInventories reads stock from PostgreSQL on every detail request. The
// catalogue can be cached, but authoritative inventory must never be cached.
func (r *ProductRepository) AttachInventories(ctx context.Context, product *model.Product) error {
	if len(product.SKUs) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(product.SKUs))
	for i := range product.SKUs {
		ids = append(ids, product.SKUs[i].ID)
		product.SKUs[i].Inventory = nil
	}
	var inventories []model.Inventory
	if err := r.db.WithContext(ctx).Where("sku_id IN ?", ids).Find(&inventories).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not load inventory", err)
	}
	bySKU := make(map[uint]*model.Inventory, len(inventories))
	for i := range inventories {
		bySKU[inventories[i].SKUID] = &inventories[i]
	}
	for i := range product.SKUs {
		product.SKUs[i].Inventory = bySKU[product.SKUs[i].ID]
	}
	return nil
}

// Create inserts a product.
func (r *ProductRepository) Create(ctx context.Context, product *model.Product) error {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		return translate(err, "could not create product")
	}
	return nil
}

// Save persists changes to an existing product.
func (r *ProductRepository) Save(ctx context.Context, product *model.Product) error {
	if err := r.db.WithContext(ctx).Save(product).Error; err != nil {
		return translate(err, "could not update product")
	}
	return nil
}

// Delete soft-removes a product.
func (r *ProductRepository) Delete(ctx context.Context, product *model.Product) error {
	if err := r.db.WithContext(ctx).Delete(product).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not delete product", err)
	}
	return nil
}

// orderFor translates a public sort token into a deterministic ORDER BY clause.
// Unknown tokens fall back to featured-first ordering so that a crafted query
// string can never change the sort in an unexpected way.
//
// Prices are stored as strings to avoid floating point drift on the client, so
// sorting casts to REAL — the only numeric cast name both PostgreSQL and
// SQLite accept with identical semantics.
func orderFor(sort string) string {
	switch sort {
	case "price_asc":
		return "(SELECT MIN(ps.price) FROM product_skus ps WHERE ps.product_id = products.id AND ps.status = 'active') ASC, id DESC"
	case "price_desc":
		return "(SELECT MIN(ps.price) FROM product_skus ps WHERE ps.product_id = products.id AND ps.status = 'active') DESC, id DESC"
	case "sales":
		return "sales DESC, id DESC"
	case "newest":
		return "id DESC"
	default:
		return "featured DESC, id DESC"
	}
}

// escapeLike neutralises wildcards in user supplied keywords so that a search
// for "50%" does not turn into a full-table scan.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// FavoriteRepository persists user bookmarks.
type FavoriteRepository struct {
	db *gorm.DB
}

// NewFavoriteRepository builds a favorite repository.
func NewFavoriteRepository(db *gorm.DB) *FavoriteRepository { return &FavoriteRepository{db: db} }

// Exists reports whether a bookmark is present.
func (r *FavoriteRepository) Exists(ctx context.Context, userID, productID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Favorite{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count).Error
	if err != nil {
		return false, apierr.Wrap(apierr.CodeInternal, "could not read favorites", err)
	}
	return count > 0, nil
}

// Set creates or removes a bookmark and reports the resulting state.
func (r *FavoriteRepository) Set(ctx context.Context, userID, productID uint, favorite bool) (bool, error) {
	if favorite {
		record := model.Favorite{UserID: userID, ProductID: productID}
		// Upsert so that a double submit cannot raise a duplicate key error.
		err := r.db.WithContext(ctx).
			Where(model.Favorite{UserID: userID, ProductID: productID}).
			Assign(model.Favorite{UserID: userID, ProductID: productID}).
			FirstOrCreate(&record).Error
		if err != nil {
			return false, apierr.Wrap(apierr.CodeInternal, "could not save favorite", err)
		}
		return true, nil
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.Favorite{}).Error; err != nil {
		return false, apierr.Wrap(apierr.CodeInternal, "could not remove favorite", err)
	}
	return false, nil
}

// ListProductIDs returns every product the user bookmarked, newest first.
func (r *FavoriteRepository) ListProductIDs(ctx context.Context, userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&model.Favorite{}).
		Where("user_id = ?", userID).
		Order("id DESC").
		Pluck("product_id", &ids).Error
	if err != nil {
		return nil, apierr.Wrap(apierr.CodeInternal, "could not load favorites", err)
	}
	return ids, nil
}

// translate maps a GORM error onto an application error.
func translate(err error, notFoundMessage string) error {
	switch {
	case isNotFound(err):
		return apierr.NotFound(notFoundMessage)
	case isDuplicate(err):
		return apierr.Conflict("resource already exists")
	default:
		return apierr.Wrap(apierr.CodeInternal, "database request failed", err)
	}
}
