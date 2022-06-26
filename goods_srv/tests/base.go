package main

import (
	"google.golang.org/grpc"
	"mxshop_srvs/goods_srv/proto"
	"mxshop_srvs/goods_srv/tests/test"
)

var conn *grpc.ClientConn

func Init() {
	var err error
	conn, err = grpc.Dial("127.0.0.1:50051", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	test.BrandClient = proto.NewGoodsClient(conn)
}

func main() {
	Init()
	//inventory.TestGetCategoryList()
	//TestCreateUser()
	test.TestGetSubCategoryList()
	conn.Close()
}
