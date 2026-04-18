package store

import "time"

type CostInstance struct {
	ID                string `gorm:"primaryKey;column:id"`
	TargetResourceID  string `gorm:"uniqueIndex;column:target_resource_id"`
	ClusterID         string `gorm:"column:cluster_id"`
	KokuSourceUUID    string `gorm:"column:koku_source_uuid"`
	KokuCostModelUUID string `gorm:"column:koku_cost_model_uuid"`
	Status            string `gorm:"column:status;index"`
	StatusMessage     string `gorm:"column:status_message"`
	Name              string `gorm:"column:name"`
	SpecJSON          string `gorm:"column:spec_json;type:text"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
