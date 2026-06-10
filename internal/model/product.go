package model

import (
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
)

// 产品状态字典
var ProductStatusDict modelDict.Dict = map[uint8]string{
	1: "启用",
	0: "禁用",
}

// Product 产品表（示例：演示数据权限接入）
type Product struct {
	ContainsDeleteBaseModel
	Name        string     `json:"name" gorm:"column:name;type:varchar(200);not null;default:'';comment:产品名称"`
	Description string     `json:"description" gorm:"column:description;type:text;comment:产品描述"`
	Price       int64      `json:"price" gorm:"column:price;type:bigint;not null;default:0;comment:价格(分)"`
	Status      uint8      `json:"status" gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:1启用,0禁用"`
	DeptId      uint       `json:"dept_id" gorm:"column:dept_id;type:int unsigned;not null;default:0;index;comment:归属部门ID"`
	CreatedBy   uint       `json:"created_by" gorm:"column:created_by;type:int unsigned;not null;default:0;index;comment:创建者UID"`
	Department  Department `json:"department" gorm:"foreignKey:DeptId;references:ID"`
}

func NewProduct() *Product {
	return BindModel(&Product{})
}

func (m *Product) TableName() string {
	return "product"
}

func (m *Product) StatusMap() string {
	return ProductStatusDict.Map(m.Status)
}

// Create 创建产品记录。
func (m *Product) Create() error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(m).Error
}
