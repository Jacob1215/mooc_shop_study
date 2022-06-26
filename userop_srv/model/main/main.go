package main

import (
	"crypto/md5"
	"encoding/hex"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"io"
	"log"
	"mxshop_srvs/userop_srv/model"
	"os"
	"time"
)

//genMd5 defines a function to generate Md5(code).
func genMd5(code string) string {
	Md5 := md5.New()
	_, _ = io.WriteString(Md5, code)
	return hex.EncodeToString(Md5.Sum(nil))
}

func main() {

	dsn := "root:root@tcp(192.168.1.104:3306)/mxshop_userop_srv?charset=utf8mb4&parseTime=True&loc=Local" //虚拟机的地址
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), //io wirter
		logger.Config{
			SlowThreshold: time.Second, //慢SQL阈值
			LogLevel:      logger.Info, //log level
			Colorful:      true,        //禁用彩色打印
		},
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, //使生成表的时候使user,不是users。
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}

	////定义一个表结构，将表结构直接生成对应的表-migrations
	////迁移schema
	_ = db.AutoMigrate(&model.LeavingMessages{}, &model.UserFav{}, &model.Address{})

}
