package initialize

import (
	"fmt"
	_ "github.com/mbobakov/grpc-consul-resolver" // It's important//差点没写这个
	"go.uber.org/zap"

	"google.golang.org/grpc"
	"mxshop_srvs/order_srv/global"
	"mxshop_srvs/order_srv/proto"
)

//第三方微服务的连接的client
//重写的Init
func InitSrvConn() {
	consulInfo := global.ServerConfig.ConsulInfo
	goodsConn, err := grpc.Dial( //拨号得按照它的写。
		fmt.Sprintf("consul://%s:%d/%s?wait=14s", consulInfo.Host, consulInfo.Port,
			global.ServerConfig.GoodsSrvInfo.Name),
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`), //目前只使用round_robin
	)
	if err != nil {
		zap.S().Fatal("[InitSrvConn]连接【商品服务失败】")
	}
	global.GoodsSrvClient = proto.NewGoodsClient(goodsConn)
	//初始化库存服务连接//这里好像可以优化
	InvConn, err := grpc.Dial( //拨号得按照它的写。
		fmt.Sprintf("consul://%s:%d/%s?wait=14s", consulInfo.Host, consulInfo.Port,
			global.ServerConfig.InventorySrvInfo.Name),
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`), //目前只使用round_robin
	)
	if err != nil {
		zap.S().Fatal("[InitSrvConn]连接【库存服务失败】")
	}
	global.InventorySrvClient = proto.NewInventoryClient(InvConn)
}
