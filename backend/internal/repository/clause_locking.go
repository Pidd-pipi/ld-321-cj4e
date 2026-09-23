package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClausesLocking 返回 SELECT ... FOR UPDATE 子句，仅在事务内生效。
func ClausesLocking() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}

// firstForUpdate 按条件查询并加行级锁（MySQL InnoDB 下为 SELECT ... FOR UPDATE）。
// 其他方言（如测试用 SQLite）不支持该子句，自动退化为普通查询。
func firstForUpdate(tx *gorm.DB, dest interface{}, query string, args ...interface{}) *gorm.DB {
	stmt := tx
	if tx.Dialector.Name() == "mysql" {
		stmt = tx.Clauses(ClausesLocking())
	}
	return stmt.First(dest, append([]interface{}{query}, args...)...)
}
