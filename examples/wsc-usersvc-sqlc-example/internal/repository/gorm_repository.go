package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GORMRepository struct {
	db *gorm.DB
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

var _ UserRepository = (*GORMRepository)(nil)
var _ OrderRepository = (*GORMRepository)(nil)

type gormUser struct {
	ID          int64          `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string         `gorm:"column:name"`
	Email       string         `gorm:"column:email"`
	Username    string         `gorm:"column:username"`
	PhoneNumber sql.NullString `gorm:"column:phone_number"`
	CreatedAt   time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (gormUser) TableName() string {
	return "users"
}

type gormOrder struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      int64     `gorm:"column:user_id"`
	Number      string    `gorm:"column:number"`
	Status      string    `gorm:"column:status"`
	TotalAmount int64     `gorm:"column:total_amount"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (gormOrder) TableName() string {
	return "orders"
}

func (r *GORMRepository) CreateUser(ctx context.Context, user User) (User, error) {
	record := gormUser{
		Name:     user.Name,
		Email:    user.Email,
		Username: user.Username,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return User{}, err
	}
	return toDomainUser(record), nil
}

func (r *GORMRepository) GetUser(ctx context.Context, id int64) (User, error) {
	var record gormUser
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return toDomainUser(record), nil
}

func (r *GORMRepository) ListUsers(ctx context.Context) ([]User, error) {
	var records []gormUser
	if err := r.db.WithContext(ctx).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	users := make([]User, 0, len(records))
	for _, record := range records {
		users = append(users, toDomainUser(record))
	}
	return users, nil
}

func (r *GORMRepository) FindByUsername(ctx context.Context, username string) (User, bool, error) {
	var record gormUser
	if err := r.db.WithContext(ctx).First(&record, "username = ?", username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return toDomainUser(record), true, nil
}

func (r *GORMRepository) UpdateUser(ctx context.Context, user User) (User, error) {
	updates := map[string]any{
		"name":       user.Name,
		"email":      user.Email,
		"username":   user.Username,
		"updated_at": time.Now().UTC(),
	}
	result := r.db.WithContext(ctx).Model(&gormUser{}).Where("id = ?", user.ID).Updates(updates)
	if result.Error != nil {
		return User{}, result.Error
	}
	if result.RowsAffected == 0 {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (r *GORMRepository) DeleteUser(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormUser{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMRepository) CreateOrder(ctx context.Context, order Order) (Order, error) {
	record := gormOrder{
		UserID:      order.UserID,
		Number:      order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Order{}, err
	}
	return toDomainOrder(record), nil
}

func (r *GORMRepository) GetOrder(ctx context.Context, id int64) (Order, error) {
	var record gormOrder
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Order{}, ErrNotFound
		}
		return Order{}, err
	}
	return toDomainOrder(record), nil
}

func (r *GORMRepository) ListOrders(ctx context.Context) ([]Order, error) {
	var records []gormOrder
	if err := r.db.WithContext(ctx).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	orders := make([]Order, 0, len(records))
	for _, record := range records {
		orders = append(orders, toDomainOrder(record))
	}
	return orders, nil
}

func (r *GORMRepository) FindByNumber(ctx context.Context, number string) (Order, bool, error) {
	var record gormOrder
	if err := r.db.WithContext(ctx).First(&record, "number = ?", number).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Order{}, false, nil
		}
		return Order{}, false, err
	}
	return toDomainOrder(record), true, nil
}

func (r *GORMRepository) UpdateOrder(ctx context.Context, order Order) (Order, error) {
	updates := map[string]any{
		"user_id":      order.UserID,
		"number":       order.Number,
		"status":       order.Status,
		"total_amount": order.TotalAmount,
		"updated_at":   time.Now().UTC(),
	}
	result := r.db.WithContext(ctx).Model(&gormOrder{}).Where("id = ?", order.ID).Updates(updates)
	if result.Error != nil {
		return Order{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Order{}, ErrNotFound
	}
	return order, nil
}

func (r *GORMRepository) DeleteOrder(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormOrder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func toDomainUser(record gormUser) User {
	return User{
		ID:       record.ID,
		Name:     record.Name,
		Email:    record.Email,
		Username: record.Username,
	}
}

func toDomainOrder(record gormOrder) Order {
	return Order{
		ID:          record.ID,
		UserID:      record.UserID,
		Number:      record.Number,
		Status:      record.Status,
		TotalAmount: record.TotalAmount,
	}
}
