package mysql

import "gorm.io/gorm/clause"

func gormExpr(expression string) clause.Expr {
	return clause.Expr{SQL: expression}
}
