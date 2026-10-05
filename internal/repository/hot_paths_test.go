package repository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestEscapeLike(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", ""},
		{"商品", "商品"},
		{`50%_off\sale`, `50\%\_off\\sale`},
		{`\%_`, `\\\%\_`},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := escapeLike(test.input); got != test.want {
				t.Errorf("escapeLike(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestEscapeLikeConcurrent(t *testing.T) {
	var group sync.WaitGroup
	for range 20 {
		group.Go(func() {
			for range 100 {
				if got := escapeLike(`50%_\`); got != `50\%\_\\` {
					t.Errorf("escapeLike concurrent = %q, want %q", got, `50\%\_\\`)
				}
			}
		})
	}
	group.Wait()
}

func openHotPathDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := database.Open(database.Options{
		Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "hot-paths.db"), AutoMigrate: true, Quiet: true,
	})
	if err != nil {
		t.Fatalf("open benchmark database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

type queryCounter struct {
	logger.Interface
	queries int
}

func (counter *queryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	counter.queries++
}

func TestCouponUsageCountsPreserveActiveStatusesAndUserScope(t *testing.T) {
	db := openHotPathDB(t)
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
	for _, test := range []struct {
		coupon, user uint
		total, own   int64
	}{
		{1, 7, 3, 2}, {1, 8, 3, 1}, {1, 9, 3, 0}, {2, 7, 1, 1}, {3, 7, 0, 0},
	} {
		t.Run(fmt.Sprintf("coupon=%d/user=%d", test.coupon, test.user), func(t *testing.T) {
			counter := &queryCounter{Interface: logger.Discard}
			repo := NewCommerceRepository(db.Session(&gorm.Session{Logger: counter}))
			total, own, err := repo.CouponUsageCounts(context.Background(), test.coupon, test.user)
			if err != nil {
				t.Fatal(err)
			}
			if total != test.total || own != test.own {
				t.Errorf("CouponUsageCounts(%d, %d) = (%d, %d), want (%d, %d)",
					test.coupon, test.user, total, own, test.total, test.own)
			}
			if counter.queries != 1 {
				t.Errorf("CouponUsageCounts query budget = %d, want 1 database round trip", counter.queries)
			}
		})
	}
}

func TestCouponUsageCountsHonorsCancelledContext(t *testing.T) {
	repo := NewCommerceRepository(openHotPathDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := repo.CouponUsageCounts(ctx, 1, 7)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CouponUsageCounts(cancelled) error = %v, want context.Canceled", err)
	}
}

func BenchmarkEscapeLike(b *testing.B) {
	for _, input := range []string{"wireless headphones", `50%_off\sale`} {
		b.Run(input, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = escapeLike(input)
			}
		})
	}
}

func BenchmarkCouponUsageCounts(b *testing.B) {
	db := openHotPathDB(b)
	usages := make([]model.CouponUsage, 1000)
	for index := range usages {
		usages[index] = model.CouponUsage{CouponID: 1, UserID: uint(index%10 + 1), OrderID: uint(index + 1), Status: model.CouponUsageReserved}
	}
	if err := db.CreateInBatches(&usages, 100).Error; err != nil {
		b.Fatal(err)
	}
	repo := NewCommerceRepository(db)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := repo.CouponUsageCounts(context.Background(), 1, 7); err != nil {
			b.Fatal(err)
		}
	}
}
