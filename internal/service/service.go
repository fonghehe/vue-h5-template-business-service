// Package service holds the business rules of the application. It is the only
// layer allowed to make decisions; handlers translate HTTP to service calls and
// nothing more.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

// Page is the paginated envelope described by the ProductPage schema.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	HasMore  bool  `json:"hasMore"`
}

// NewPage builds a page envelope from a slice and the total row count.
func NewPage[T any](items []T, total int64, page, pageSize int) Page[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return Page[T]{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}
}

// FavoriteResult is the response of a favorite toggle.
type FavoriteResult struct {
	ProductID uint `json:"productId"`
	Favorite  bool `json:"favorite"`
}

// Container wires every service together.
type Container struct {
	Config config.Config
	Auth   *AuthService
	Users  *UserService
	// Products is exported so the HTTP layer can also serve operator routes.
	Products  *ProductService
	Favorites *FavoriteService
}

// New builds the service container.
func New(cfg config.Config, db *gorm.DB) *Container {
	users := repository.NewUserRepository(db)
	products := repository.NewProductRepository(db)
	favorites := repository.NewFavoriteRepository(db)
	issuer := auth.New(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)

	return &Container{
		Config:    cfg,
		Auth:      NewAuthService(cfg, users, issuer),
		Users:     NewUserService(users),
		Products:  NewProductService(products),
		Favorites: NewFavoriteService(products, favorites),
	}
}

// AuthService handles credential exchange and token rotation.
type AuthService struct {
	config config.Config
	users  *repository.UserRepository
	issuer *auth.Issuer
	// verifyUser is overridable in tests to avoid bcrypt cost.
	verifyUser func(hash, password string) bool
}

// NewAuthService builds an authentication service.
func NewAuthService(cfg config.Config, users *repository.UserRepository, issuer *auth.Issuer) *AuthService {
	return &AuthService{config: cfg, users: users, issuer: issuer, verifyUser: comparePassword}
}

// LoginResult is the payload returned by login and refresh.
type LoginResult struct {
	model.User
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"`
}

// Login validates credentials and mints an access/refresh token pair.
func (s *AuthService) Login(ctx context.Context, username, password string, issueRefresh bool) (LoginResult, auth.TokenPair, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResult{}, auth.TokenPair{}, apierr.Validation("username and password are required")
	}

	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		// Do not reveal whether the account exists. The bcrypt comparison below
		// still runs against a dummy hash so that timing stays flat.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		var apiErr *apierr.Error
		if errors.As(err, &apiErr) && apiErr.Code == apierr.CodeNotFound {
			return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("username or password is incorrect")
		}
		return LoginResult{}, auth.TokenPair{}, err
	}

	if !s.verifyUser(user.PasswordHash, password) {
		return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("username or password is incorrect")
	}
	if user.Status != "active" {
		return LoginResult{}, auth.TokenPair{}, apierr.Forbidden("account is disabled")
	}

	pair, err := s.issuer.Issue(user.ID, user.Username, user.RealName, user.Roles,
		s.config.AccessTokenTTL, s.config.RefreshTokenTTL, issueRefresh)
	if err != nil {
		return LoginResult{}, auth.TokenPair{}, apierr.Wrap(apierr.CodeInternal, "could not issue token", err)
	}
	return LoginResult{User: user.Public(), AccessToken: pair.AccessToken, ExpiresIn: pair.ExpiresIn}, pair, nil
}

// Refresh exchanges a refresh token for a fresh access token.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (LoginResult, auth.TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("refresh token is required")
	}
	claims, err := s.issuer.Verify(refreshToken, auth.RefreshToken)
	if err != nil {
		return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("invalid or expired refresh token")
	}
	userID, err := claims.UserID()
	if err != nil {
		return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("invalid refresh token subject")
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("account no longer exists")
	}
	if user.Status != "active" {
		return LoginResult{}, auth.TokenPair{}, apierr.Forbidden("account is disabled")
	}

	pair, err := s.issuer.Issue(user.ID, user.Username, user.RealName, user.Roles,
		s.config.AccessTokenTTL, s.config.RefreshTokenTTL, true)
	if err != nil {
		return LoginResult{}, auth.TokenPair{}, apierr.Wrap(apierr.CodeInternal, "could not issue token", err)
	}
	return LoginResult{User: user.Public(), AccessToken: pair.AccessToken, ExpiresIn: pair.ExpiresIn}, pair, nil
}

// VerifyAccessToken validates a bearer token and returns its claims.
func (s *AuthService) VerifyAccessToken(token string) (*auth.Claims, error) {
	claims, err := s.issuer.Verify(token, auth.AccessToken)
	if err != nil {
		return nil, apierr.Unauthorized("invalid or expired token")
	}
	return claims, nil
}

// dummyHash keeps the failure path of a login attempt as expensive as the
// success path, so response timing cannot be used to enumerate accounts.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func comparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// UserService exposes account reads.
type UserService struct {
	users *repository.UserRepository
}

// NewUserService builds a user service.
func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

// ByID returns the public representation of an account.
func (s *UserService) ByID(ctx context.Context, id uint) (model.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	return user.Public(), nil
}

// ProductService exposes catalog reads and writes.
type ProductService struct {
	products *repository.ProductRepository
}

// NewProductService builds a product service.
func NewProductService(products *repository.ProductRepository) *ProductService {
	return &ProductService{products: products}
}

// ProductQuery is the public, validated form of a list request.
type ProductQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Sort     string
	// IncludeHidden is operator-only; the HTTP layer gates it behind admin.
	IncludeHidden bool
	Status        string
	Featured      *bool
}

// List returns one page of the catalog.
func (s *ProductService) List(ctx context.Context, query ProductQuery) (Page[model.Product], error) {
	items, total, err := s.products.List(ctx, repository.ProductListQuery{
		Page:          query.Page,
		PageSize:      query.PageSize,
		Keyword:       query.Keyword,
		Sort:          query.Sort,
		IncludeHidden: query.IncludeHidden,
		Status:        query.Status,
		Featured:      query.Featured,
	})
	if err != nil {
		return Page[model.Product]{}, err
	}
	return NewPage(items, total, query.Page, query.PageSize), nil
}

// Detail returns a product for public consumption.
func (s *ProductService) Detail(ctx context.Context, id uint) (model.Product, error) {
	return s.products.FindPublicByID(ctx, id)
}

// AdminDetail returns a product regardless of status; operator-only.
func (s *ProductService) AdminDetail(ctx context.Context, id uint) (model.Product, error) {
	return s.products.FindByID(ctx, id)
}

// ProductInput is the operator payload for creating or updating a product.
type ProductInput struct {
	Title       string
	ImgURL      string
	Price       string
	VipPrice    string
	ShopDesc    string
	Delivery    string
	ShopName    string
	Description string
	Stock       int
	Status      string
	Featured    bool
}

// Create adds a product to the catalog.
func (s *ProductService) Create(ctx context.Context, input ProductInput) (model.Product, error) {
	product := productFromInput(input)
	product.SearchText = searchText(product)
	if err := validateProduct(product); err != nil {
		return model.Product{}, err
	}
	if err := s.products.Create(ctx, &product); err != nil {
		return model.Product{}, err
	}
	return product, nil
}

// Update mutates an existing product.
func (s *ProductService) Update(ctx context.Context, id uint, input ProductInput) (model.Product, error) {
	product, err := s.products.FindByID(ctx, id)
	if err != nil {
		return model.Product{}, err
	}
	updated := productFromInput(input)
	updated.ID = product.ID
	updated.Sales = product.Sales
	updated.CreatedAt = product.CreatedAt
	updated.SearchText = searchText(updated)
	if err := validateProduct(updated); err != nil {
		return model.Product{}, err
	}
	if err := s.products.Save(ctx, &updated); err != nil {
		return model.Product{}, err
	}
	return updated, nil
}

// Delete soft-removes a product.
func (s *ProductService) Delete(ctx context.Context, id uint) error {
	product, err := s.products.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return s.products.Delete(ctx, &product)
}

func productFromInput(input ProductInput) model.Product {
	return model.Product{
		Title:       strings.TrimSpace(input.Title),
		ImgURL:      strings.TrimSpace(input.ImgURL),
		Price:       strings.TrimSpace(input.Price),
		VipPrice:    strings.TrimSpace(input.VipPrice),
		ShopDesc:    strings.TrimSpace(input.ShopDesc),
		Delivery:    strings.TrimSpace(input.Delivery),
		ShopName:    strings.TrimSpace(input.ShopName),
		Description: strings.TrimSpace(input.Description),
		Stock:       input.Stock,
		Status:      input.Status,
		Featured:    input.Featured,
	}
}

// validateProduct enforces the invariants that the database cannot express
// portably across PostgreSQL and SQLite.
func validateProduct(product model.Product) error {
	switch {
	case product.Title == "":
		return apierr.Validation("title is required")
	case product.ImgURL == "":
		return apierr.Validation("imgUrl is required")
	case !isNumeric(product.Price):
		return apierr.Validation("price must be a non-negative number")
	case !isNumeric(product.VipPrice):
		return apierr.Validation("vipPrice must be a non-negative number")
	case product.ShopName == "":
		return apierr.Validation("shopName is required")
	case product.Status == "" || !validStatus(product.Status):
		return apierr.Validation("status must be one of draft, on_sale, sold_out")
	}
	return nil
}

func validStatus(status string) bool {
	switch status {
	case model.StatusDraft, model.StatusOnSale, model.StatusSoldOut:
		return true
	}
	return false
}

// isNumeric accepts plain decimal strings, which is what the API documents for
// price fields. Storing money as text keeps the client free of float drift.
func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	dots := 0
	for index := 0; index < len(value); index++ {
		switch char := value[index]; {
		case char >= '0' && char <= '9':
		case char == '.':
			dots++
			if dots > 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func searchText(product model.Product) string {
	return strings.Join([]string{product.Title, product.ShopName, product.ShopDesc, product.Description}, " ")
}

// FavoriteService manages the many-to-many bookmark relation.
type FavoriteService struct {
	products  *repository.ProductRepository
	favorites *repository.FavoriteRepository
}

// NewFavoriteService builds a favorite service.
func NewFavoriteService(products *repository.ProductRepository, favorites *repository.FavoriteRepository) *FavoriteService {
	return &FavoriteService{products: products, favorites: favorites}
}

// Set toggles a bookmark, verifying the product exists and is visible first.
func (s *FavoriteService) Set(ctx context.Context, userID, productID uint, favorite bool) (FavoriteResult, error) {
	// Reject bookmarks for products the caller could not see anyway.
	if _, err := s.products.FindPublicByID(ctx, productID); err != nil {
		return FavoriteResult{}, err
	}
	state, err := s.favorites.Set(ctx, userID, productID, favorite)
	if err != nil {
		return FavoriteResult{}, err
	}
	return FavoriteResult{ProductID: productID, Favorite: state}, nil
}

// List returns the bookmarked products of a user, newest first.
func (s *FavoriteService) List(ctx context.Context, userID uint, page, pageSize int) (Page[model.Product], error) {
	ids, err := s.favorites.ListProductIDs(ctx, userID)
	if err != nil {
		return Page[model.Product]{}, err
	}
	total := int64(len(ids))
	start := (page - 1) * pageSize
	if start > len(ids) {
		return NewPage([]model.Product{}, total, page, pageSize), nil
	}
	end := start + pageSize
	if end > len(ids) {
		end = len(ids)
	}

	items := make([]model.Product, 0, end-start)
	for _, id := range ids[start:end] {
		product, err := s.products.FindByID(ctx, id)
		if err != nil {
			// A bookmark pointing at a soft-deleted product is stale; skip it
			// instead of failing the whole page.
			continue
		}
		items = append(items, product)
	}
	return NewPage(items, total, page, pageSize), nil
}

// TimeNow is a seam so that tests can freeze time without touching the clock.
var TimeNow = time.Now
