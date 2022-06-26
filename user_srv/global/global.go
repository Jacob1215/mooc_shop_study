package global

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"log"
	"mxshop_srvs/user_srv/config"
	"os"
	"time"
)

//定义全局变量
var (
	DB           *gorm.DB
	ServerConfig config.ServerConfig
	NacosConfig  config.NacosConfig
)

func init() {
	dsn := "root:root@tcp(192.168.1.104:3306)/mxshop_user_srv?charset=utf8mb4&parseTime=True&loc=Local" //虚拟机的地址
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), //io wirter
		logger.Config{
			SlowThreshold: time.Second, //慢SQL阈值
			LogLevel:      logger.Info, //log level
			Colorful:      true,        //禁用彩色打印
		},
	)
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, //使生成表的时候使user,不是users。
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}
}
