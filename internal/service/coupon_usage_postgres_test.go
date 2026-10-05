package service

import (
	"context"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

// Keep the conditional aggregate covered on the runtime SQL dialect too.
func TestPostgresCouponUsageCounts(t *testing.T) {
	db := newPostgresCommerceDB(t)
	repo := repository.NewCommerceRepository(db)
	ctx := context.Background()
	total, own, err := repo.CouponUsageCounts(ctx, 1, 7)
	if err != nil || total != 0 || own != 0 {
		t.Fatalf("CouponUsageCounts(empty) = (%d, %d, %v), want (0, 0, nil)", total, own, err)
	}
	usages := []model.CouponUsage{
		{CouponID: 1, UserID: 7, OrderID: 1, Status: model.CouponUsageReserved},
		{CouponID: 1, UserID: 7, OrderID: 2, Status: model.CouponUsageConsumed},
		{CouponID: 1, UserID: 7, OrderID: 3, Status: model.CouponUsageReleased},
		{CouponID: 1, UserID: 8, OrderID: 4, Status: model.CouponUsageReserved},
		{CouponID: 2, UserID: 7, OrderID: 5, Status: model.CouponUsageConsumed},
	}
	if err := db.Create(&usages).Error; err != nil {
		t.Fatal(err)
	}
	total, own, err = repo.CouponUsageCounts(ctx, 1, 7)
	if err != nil || total != 3 || own != 2 {
		t.Errorf("CouponUsageCounts(1, 7) = (%d, %d, %v), want (3, 2, nil)", total, own, err)
	}
	if err := db.Model(&model.CouponUsage{}).Where("coupon_id = ? AND user_id = ?", 1, 7).
		Update("status", model.CouponUsageReleased).Error; err != nil {
		t.Fatal(err)
	}
	total, own, err = repo.CouponUsageCounts(ctx, 1, 7)
	if err != nil || total != 1 || own != 0 {
		t.Errorf("CouponUsageCounts(after release) = (%d, %d, %v), want (1, 0, nil)", total, own, err)
	}
}
