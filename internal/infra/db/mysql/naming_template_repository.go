package mysql

import (
	"context"
	"time"
)

// NamingTemplateRecord 分片命名模板配置记录。
//
// 对应数据库表 t_config_naming_template，
// 按版本固化命名模板配置。
type NamingTemplateRecord struct {
	ID              uint64    `gorm:"column:id;primaryKey"`
	ConfigVersion   uint64    `gorm:"column:config_version;uniqueIndex"`
	OutputBaseTpl   string    `gorm:"column:output_base_prefix_tpl"`
	InitSegNameTpl  string    `gorm:"column:init_seg_name_tpl"`
	MediaSegNameTpl string    `gorm:"column:media_seg_name_tpl"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

// TableName 返回表名。
func (NamingTemplateRecord) TableName() string {
	return "t_config_naming_template"
}

// NamingTemplateRepository 分片命名模板仓储。
type NamingTemplateRepository struct {
	db *DB
}

// NewNamingTemplateRepository 创建命名模板仓储。
func NewNamingTemplateRepository(db *DB) *NamingTemplateRepository {
	return &NamingTemplateRepository{db: db}
}

// Save 保存命名模板记录。
func (r *NamingTemplateRepository) Save(ctx context.Context, record NamingTemplateRecord) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

// FindByVersion 按配置版本查询。
func (r *NamingTemplateRepository) FindByVersion(ctx context.Context, configVersion uint64) (NamingTemplateRecord, bool) {
	var record NamingTemplateRecord
	err := r.db.WithContext(ctx).Where("config_version = ?", configVersion).First(&record).Error
	if err != nil {
		return NamingTemplateRecord{}, false
	}
	return record, true
}

// Latest 获取最新版本。
func (r *NamingTemplateRepository) Latest(ctx context.Context) (NamingTemplateRecord, bool) {
	var record NamingTemplateRecord
	err := r.db.WithContext(ctx).Order("config_version DESC").First(&record).Error
	if err != nil {
		return NamingTemplateRecord{}, false
	}
	return record, true
}
