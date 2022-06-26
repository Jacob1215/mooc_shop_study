package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"github.com/olivere/elastic/v7"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"io"
	"log"
	"mxshop_srvs/goods_srv/global"
	"mxshop_srvs/goods_srv/model"
	"os"
	"strconv"
	"time"
)

//genMd5 defines a function to generate Md5(code).
func genMd5(code string) string {
	Md5 := md5.New()
	_, _ = io.WriteString(Md5, code)
	return hex.EncodeToString(Md5.Sum(nil))
}

func main() {
	Mysql2Es()
	//
	//dsn := "root:root@tcp(192.168.1.104:3306)/mxshop_goods_srv?charset=utf8mb4&parseTime=True&loc=Local" //虚拟机的地址
	//newLogger := logger.New(
	//	log.New(os.Stdout, "\r\n", log.LstdFlags), //io wirter
	//	logger.Config{
	//		SlowThreshold: time.Second, //慢SQL阈值
	//		LogLevel:      logger.Info, //log level
	//		Colorful:      true,        //禁用彩色打印
	//	},
	//)
	//db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
	//	NamingStrategy: schema.NamingStrategy{
	//		SingularTable: true, //使生成表的时候使user,不是users。
	//	},
	//	Logger: newLogger,
	//})
	//if err != nil {
	//	panic(err)
	//}
	//
	//////定义一个表结构，将表结构直接生成对应的表-migrations
	//////迁移schema
	//_ = db.AutoMigrate(&model.Category{}, &model.Brands{}, &model.GoodsCategoryBrand{},
	//	&model.Banner{}, &model.Goods{})

}

func Mysql2Es() {
	dsn := "root:root@tcp(192.168.1.104:3306)/mxshop_goods_srv?charset=utf8mb4&parseTime=True&loc=Local" //虚拟机的地址
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
	host := "http://192.168.1.104:9200"
	//host := fmt.Sprintf("http://%s:%d", global.ServerConfig.EsInfo.Host, global.ServerConfig.EsInfo.Port)
	logger1 := log.New(os.Stdout, "mxshop_jacob", log.LstdFlags)
	global.EsClient, err = elastic.NewClient(elastic.SetURL(host), elastic.SetSniff(false), elastic.SetTraceLog(logger1))
	if err != nil {
		// Handle error
		panic(err)
	}

	var goods []model.Goods
	db.Find(&goods)
	for _, g := range goods {
		esModel := model.EsGoods{
			ID:          g.ID,
			CategoryID:  g.CategoryID,
			BrandsID:    g.BrandsID,
			OnSale:      g.OnSale,
			ShipFree:    g.ShipFree,
			IsNew:       g.IsNew,
			IsHot:       g.IsHot,
			Name:        g.Name,
			ClickNum:    g.ClickNum,
			SoldNum:     g.SoldNum,
			FavNum:      g.FavNum,
			MarketPrice: g.MarketPrice,
			GoodsBrief:  g.GoodsBrief,
			ShopPrice:   g.ShopPrice,
		}
		_, err := global.EsClient.Index().Index(esModel.GetIndexName()).BodyJson(esModel).Id(strconv.Itoa(int(g.ID))).Do(context.Background())
		if err != nil {
			panic(err)
		}
	}
	//强调一下 一定要将docker启动es的java_ops的内存设置大一些 否则运行过程中会出现 bad request错误。

}
