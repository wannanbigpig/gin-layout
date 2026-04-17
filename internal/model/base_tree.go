package model

import "gorm.io/gorm"

// HasChildren 判断指定父节点是否存在子节点。
func HasChildren[T any, M BaseModelInterface[T]](model M, pid uint) (bool, error) {
	count, err := model.Count("pid = ?", pid)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateChildrenNum 统计并更新父节点的 children_num 字段。
func UpdateChildrenNum[T any, M BaseModelInterface[T]](model M, pid uint, tx *gorm.DB) error {
	if pid == 0 {
		return nil
	}

	getDB := func() (*gorm.DB, error) {
		if tx != nil {
			return tx.Model(model), nil
		}
		return GetDB(model)
	}

	var count int64
	queryDB, err := getDB()
	if err != nil {
		return err
	}
	if err := queryDB.Where("pid = ?", pid).Count(&count).Error; err != nil {
		return err
	}

	updateDB, err := getDB()
	if err != nil {
		return err
	}
	return updateDB.Where("id = ?", pid).Update("children_num", count).Error
}
