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
	DB     *gorm.DB
	models = []interface{}{}
)

func initDB(url string) error {
	mysqlUrl := strings.TrimPrefix(url, "mysql://")
	db, err := gorm.Open(mysql.Open(mysqlUrl))

	if err != nil {
		logrus.Fatalf("Failed to connect to mysql: %+v", err)
		return err
	}
	DB = db
	return nil
}

func AddModel(model interface{}) {
	models = append(models, model)
}

func Migrate(url string) error {
	if err := initDB(url); err != nil {
		return err
	}
	return DB.AutoMigrate(
		models...,
	)
}
