package model

import (
	"database/sql/driver"
	"encoding/json"
)

//type Stock struct {//仓库，多对多的关系，会比较难。
//	BaseModel
//	Name    string
//	Address string
//}

type GoodsDetail struct {
	Goods int32
	Num   int32
}
type GoodsDetailList []GoodsDetail

func (g GoodsDetailList) Value() (driver.Value, error) {
	return json.Marshal(g)
}

func (g *GoodsDetailList) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), &g)
}

type Inventory struct {
	BaseModel
	Goods   int32 `gorm:"type:int;index"` //重点查这个，所以要加个索引
	Stocks  int32 `gorm:"type:int"`
	Version int32 `gorm:"type:int"` //分布式锁的乐观锁
}

type InventoryNew struct {
	BaseModel
	Goods   int32 `gorm:"type:int;index"` //重点查这个，所以要加个索引
	Stocks  int32 `gorm:"type:int"`
	Version int32 `gorm:"type:int"` //分布式锁的乐观锁
	Freeze  int32 `gorm:"type:int"` //冻结库存
}
type Delivery struct {
	Goods   int32  `gorm:"type:int;index"`
	Nums    int32  `gorm:"type:int"`
	OrderSn string `gorm:"type:varchar(200)"`
	Status  string `gorm:"type:varchar(200)"` //1.表示等待支付，2.表示支付成功 3.
}

type StockSellDetail struct {
	OrderSn string          `gorm:"type:varchar(200);index:idx_order_sn,unique"` //记得写索引名称，
	Status  int32           `gorm:"type:varchar(200)"`                           //1.表示已扣减，2.表示已归还。
	Detail  GoodsDetailList `gorm:"type:varchar(200)"`
}

func (StockSellDetail) TableName() string {
	return "stockselldetail"
}

//归还是分布式事务，完整性保障的过程，所以后面做。
//type InventoryHistory struct {
//	user   int32
//	goods  int32
//	nums   int32
//	order  int32
//	status int32 //1.表示库存是预扣减。幂等性？？2来表示已经支付成功了。
//}
