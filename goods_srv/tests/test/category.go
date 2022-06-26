package test

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"mxshop_srvs/goods_srv/proto"
)

func TestGetCategoryList() {
	rsp, err := BrandClient.GetAllCategorysList(context.Background(), &empty.Empty{})
	if err != nil {
		panic(err)
	}
	fmt.Println(rsp.Total)
	for _, category := range rsp.Data {
		fmt.Println(category.Name)
	}
}
func TestGetSubCategoryList() {
	rsp, err := BrandClient.GetSubCategory(context.Background(), &proto.CategoryListRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println(rsp.Total)
	fmt.Println(rsp.SubCategorys)
}
