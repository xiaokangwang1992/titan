/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/21 23:11:38
 Desc     :
*/

package storage

import (
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	gormDB *gorm.DB
	models = []interface{}{}
)

func initDB(url string) error {
	mysqlUrl := strings.TrimPrefix(url, "mysql://")
	db, err := gorm.Open(mysql.Open(mysqlUrl))

	if err != nil {
		logrus.Errorf("Failed to connect to mysql: %+v", err)
		return err
	}
	gormDB = db
	return nil
}

func RegisterModel(model interface{}) {
	models = append(models, model)
}

func DB() *gorm.DB {
	if gormDB == nil {
		panic("db is nil")
	}
	return gormDB
}

func Migrate(url string) error {
	if err := initDB(url); err != nil {
		panic(err)
	}
	return gormDB.AutoMigrate(
		models...,
	)
}
