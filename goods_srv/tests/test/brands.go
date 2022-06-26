package test

import (
	"context"
	"fmt"
	"mxshop_srvs/goods_srv/proto"
)

var BrandClient proto.GoodsClient

func TestGetBrandList() {
	rsp, err := BrandClient.BrandList(context.Background(), &proto.BrandFilterRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println(rsp.Total)
	for _, brand := range rsp.Data {
		fmt.Println(brand.Name)
	}
}
