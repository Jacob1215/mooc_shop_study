package main

import (
	"crypto/md5"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"github.com/anaskhan96/go-password-encoder"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"io"
	"log"
	"mxshop_srvs/user_srv/model"
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

	dsn := "root:root@tcp(192.168.1.104:3306)/mxshop_user_srv?charset=utf8mb4&parseTime=True&loc=Local" //虚拟机的地址
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
	//设定密码/ctrl+/就是统一注释和取消注释
	options := &password.Options{10, 100, 32, sha512.New}
	salt, encodedPwd := password.Encode("admin123", options)
	newPassword := fmt.Sprintf("$pbkdf2-sha512$%s$%s", salt, encodedPwd)
	//fmt.Println(len(newPassword)) //确保长度不超过100，不然保存进数据库会失败。
	fmt.Println(newPassword)

	for i := 0; i < 10; i++ {
		user := model.User{
			NickName: fmt.Sprintf("jacob%d", i),
			Mobile:   fmt.Sprintf("1891702615%d", i),
			Password: newPassword,
		}
		db.Save(&user)
	}

	////定义一个表结构，将表结构直接生成对应的表-migrations
	////迁移schema
	_ = db.AutoMigrate(&model.User{})

	//fmt.Println((genMd5("xxxxx_123456")))

	// Using the default options
	//salt, encodedPwd := password.Encode("generic password", nil)
	//fmt.Println(salt)
	//fmt.Println(encodedPwd)
	//check := password.Verify("generic password", salt, encodedPwd, nil) //还验证了一下，非常重要。
	//fmt.Println(check)                                                  // true

	// Using custom options

	//passwordInfo := strings.Split(newPassword, "$")
	//fmt.Println(passwordInfo)
	//check := password.Verify("generic password", passwordInfo[2], passwordInfo[3], options)
	//fmt.Println(check) // true
}
