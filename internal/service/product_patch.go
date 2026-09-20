package service

import (
	"context"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

type ProductPatchInput struct {
	Name        *string
	CategoryID  *string
	Brand       *string
	Cover       *string
	Title       *string
	ImgURL      *string
	Price       *string
	VipPrice    *string
	ShopDesc    *string
	Delivery    *string
	ShopName    *string
	Description *string
	Stock       *int
	Status      *string
	Featured    *bool
}

// Patch merges the supplied fields while holding the product row lock so
// concurrent partial updates preserve each other's untouched fields.
func (s *ProductService) Patch(ctx context.Context, id uint, input ProductPatchInput) (model.Product, error) {
	var result model.Product
	err := s.products.Transaction(ctx, func(tx *repository.ProductRepository) error {
		current, err := tx.LockByID(ctx, id)
		if err != nil {
			return err
		}
		next := productFromInput(mergeProductPatch(input, current))
		next.ID, next.Sales, next.CreatedAt = current.ID, current.Sales, current.CreatedAt
		next.SearchText = searchText(next)
		if err := validateProduct(next); err != nil {
			return err
		}
		if err := tx.Save(ctx, &next); err != nil {
			return err
		}
		result, err = tx.FindByID(ctx, id)
		return err
	})
	if err != nil {
		return model.Product{}, repository.WrapTransaction(err)
	}
	if err := s.cache.Delete(ctx, id); err != nil {
		s.logger.WarnContext(ctx, "product cache invalidation failed", "productId", id, "error", err)
	}
	return result, nil
}
func mergeProductPatch(p ProductPatchInput, existing model.Product) ProductInput {
	input := ProductInput{}
	if p.Name == nil {
		input.Name = existing.Name
	} else {
		input.Name = *p.Name
	}
	if p.CategoryID == nil {
		input.CategoryID = existing.CategoryID
	} else {
		input.CategoryID = *p.CategoryID
	}
	if p.Brand == nil {
		input.Brand = existing.Brand
	} else {
		input.Brand = *p.Brand
	}
	if p.Cover == nil {
		input.Cover = existing.Cover
	} else {
		input.Cover = *p.Cover
	}
	if p.Title == nil {
		input.Title = existing.Title
	} else {
		input.Title = *p.Title
	}
	if p.ImgURL == nil {
		input.ImgURL = existing.ImgURL
	} else {
		input.ImgURL = *p.ImgURL
	}
	if p.Price == nil {
		input.Price = existing.Price
	} else {
		input.Price = *p.Price
	}
	if p.VipPrice == nil {
		input.VipPrice = existing.VipPrice
	} else {
		input.VipPrice = *p.VipPrice
	}
	if p.ShopDesc == nil {
		input.ShopDesc = existing.ShopDesc
	} else {
		input.ShopDesc = *p.ShopDesc
	}
	if p.Delivery == nil {
		input.Delivery = existing.Delivery
	} else {
		input.Delivery = *p.Delivery
	}
	if p.ShopName == nil {
		input.ShopName = existing.ShopName
	} else {
		input.ShopName = *p.ShopName
	}
	if p.Description == nil {
		input.Description = existing.Description
	} else {
		input.Description = *p.Description
	}
	if p.Stock == nil {
		input.Stock = existing.Stock
	} else {
		input.Stock = *p.Stock
	}
	if p.Status == nil {
		input.Status = existing.Status
	} else {
		input.Status = *p.Status
	}
	if p.Featured == nil {
		input.Featured = existing.Featured
	} else {
		input.Featured = *p.Featured
	}
	return input
}
