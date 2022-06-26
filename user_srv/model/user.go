package model

import (
	"gorm.io/gorm"
	"time"
)


type BaseModel struct {//自定义model，方便加上自己的字段。
	ID int32 `gorm:"primarykey"`
	CreatedAt time.Time `gorm:"column:add_time"`
	UpdatedAt time.Time `gorm:"column:update_time"`
	DeletedAt gorm.DeletedAt
	IsDeleted bool
}

type User struct {
	BaseModel
	Mobile string `gorm:"index:idx_mobile;unique;type:varchar(11);not null"`//通过手机号码查询用户
	Password string `gorm:"type:varchar(100);not null"`
	NickName string `gorm:"type:varchar(20)"`
	Birthday *time.Time `gorm:"type:datetime"`	//这儿必须加*好，不然容易报错。
	Gender string `gorm:"column:gender;default:male;type:varchar(6) comment 'female表示女，male表示男'"`
	Role int `gorm:"column:role;default:1;type:int comment '1表示普通用户，2表示管理员'"`
}