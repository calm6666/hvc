package mysql

import (
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func isDuplicateEntryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
