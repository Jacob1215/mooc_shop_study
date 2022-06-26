package model

import (
	"context"
	"gorm.io/gorm"
	"mxshop_srvs/goods_srv/global"
	"strconv"
)

type Category struct {
	//定义表结构时，比较重要的就是字段的类型，长度，这个字段是否能为null。这个字段应该设置为可以为null还是设置为空，0.
	//实际开发过程中，尽量设置 ：不为 null
	//这些类型我们使用int32还是int。最好是int32。int64更站空间。
	BaseModel
	Name             string      `gorm:"type:varchar(20);not null" json:"name"`
	ParentCategoryID int32       `json:"parent"`
	ParentCategory   *Category   `json:"-"`                                                           //自己指向自己需要用指针。//为什么我这儿没有生成外键
	SubCategory      []*Category `gorm:"foreignKey:ParentCategory;references:ID" json:"sub_category"` //重要问题，mysql的外键是什么。
	Level            int32       `gorm:"type:int;not null;default:1" json:"level"`                    //我这边comment写错了，故给删了
	IsTab            bool        `gorm:"default:false;not null" json:"is_tab"`
}

//type Category2 struct {
//	Name             string    `gorm:"type:varchar(20);not null"`
//	ParentCategoryID int32     //这个就是外键，就是我没有自动生成外键。
//	ParentCategory   *Category //自己指向自己需要用指针。//为什么我这儿没有生成外键
//}

type Brands struct { //品牌表，Brands
	BaseModel
	Name string `gorm:"type:varchar(20);not null"`
	Logo string `gorm:"type:varchar(200);not null;default:''"` //有些品牌可能会没有logo。
}

type GoodsCategoryBrand struct { //这里建议自己定义这张表。
	BaseModel
	CategoryID int32 `gorm:"type:int;index:idx_category_brand,unique"` //两个index索引一样，就可以建成联合的唯一索引。
	Category   Category

	BrandsID int32 `gorm:"type:int;index:idx_category_brand,unique"`
	Brands   Brands
}

type Banner struct { //轮播图
	BaseModel
	Image string `gorm:"type:varchar(200);not null"`
	Url   string `gorm:"type:varchar(200);not null"`
	Index int32  `gorm:"type:int;default:1;not null"`
}

type Goods struct { //商品表
	BaseModel
	CategoryID int32 `gorm:"type:int;not null"` //这个就不建立唯一索引
	Category   Category

	BrandsID int32 `gorm:"type:int;not null"`
	Brands   Brands

	OnSale   bool `gorm:"default:false;not null"`
	ShipFree bool `gorm:"default:false;not null"`
	IsNew    bool `gorm:"default:false;not null"`
	IsHot    bool `gorm:"default:false;not null"`

	Name        string  `gorm:"type:varchar(50);not null"`
	GoodsSn     string  `gorm:"type:varchar(50);not null"` //商家方产品的编号。
	ClickNum    int32   `gorm:"type:int;default:0;not null"`
	SoldNum     int32   `gorm:"type:int;default:0;not null"`
	FavNum      int32   `gorm:"type:int;default:0;not null"` //收藏
	MarketPrice float32 `gorm:"not null"`
	ShopPrice   float32 `gorm:"not null"`
	GoodsBrief  string  `gorm:"type:varchar(100);not null"`
	//自定义类型
	Images          GormList `gorm:"type:varchar(1000);not null"` //自定义类型
	DescImages      GormList `gorm:"type:varchar(1000);not null"`
	GoodsFrontImage string   `gorm:"type:varchar(200);not null"`
}

func (GoodsCategoryBrand) TableName() string { //直接不生成下划线的方式，自己定义表明。喜欢下划线，就不重载表名。
	return "goodscategorybrand"
}

func (g *Goods) AfterCreate(tx *gorm.DB) (err error) { //这样就可以达到一个保存的 效果。
	esModel := EsGoods{
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
	_, err = global.EsClient.Index().Index(esModel.GetIndexName()).BodyJson(esModel).
		Id(strconv.Itoa(int(g.ID))).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}
