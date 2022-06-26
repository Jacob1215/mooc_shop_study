package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/api"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"mxshop_srvs/goods_srv/global"
	"mxshop_srvs/goods_srv/handler"
	"mxshop_srvs/goods_srv/initialize"
	"mxshop_srvs/goods_srv/proto"
	"mxshop_srvs/goods_srv/utils"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// String defines a string flag with specified name, default value, and usage string.
	// The return value is the address of a string variable that stores the value of the flag.
	IP := flag.String("ip", "0.0.0.0", "ip地址")
	Port := flag.Int("port", 50051, "端口号")
	//初始化
	initialize.InitLogger()
	initialize.InitConfig()
	initialize.InitDB()
	initialize.InitEs()
	zap.S().Info(global.ServerConfig)
	flag.Parse()
	zap.S().Info("ip:", *IP)
	if *Port == 0 {
		*Port, _ = utils.GetFreePort()
	}
	zap.S().Info("port:", *Port)
	server := grpc.NewServer()
	proto.RegisterGoodsServer(server, &handler.GoodsServer{})
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *IP, *Port))
	if err != nil {
		panic("failed to listen:" + err.Error())
	}
	//注册服务健康检查
	grpc_health_v1.RegisterHealthServer(server, health.NewServer()) //不需要用&服务，这样做就可以了。
	//服务注册
	cfg := api.DefaultConfig()
	cfg.Address = fmt.Sprintf("%s:%d",
		global.ServerConfig.ConsulInfo.Host, global.ServerConfig.ConsulInfo.Port) //改端口号
	client, err := api.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	//生成对应的检查对象。
	check := &api.AgentServiceCheck{
		GRPC: fmt.Sprintf("%s:%d", global.ServerConfig.Host, *Port), //这个端口号一定要改，不然容易出错
		//这个url需先配置好，不然健康检查不能通过。
		//host和port先这样写，后期从配置中心再去拿。让两边保持不一致。//这个是给web层的。
		Timeout:                        "5s", //5秒超时，5秒检查
		Interval:                       "5s",
		DeregisterCriticalServiceAfter: "10s",
	}

	//生成注册对象
	registration := new(api.AgentServiceRegistration)
	registration.Name = global.ServerConfig.Name
	serviceID := fmt.Sprintf("%s", uuid.NewV4())
	registration.ID = serviceID
	registration.Port = *Port
	registration.Tags = global.ServerConfig.Tags    //这个地方可以配置，不用写死
	registration.Address = global.ServerConfig.Host //这个是拿来做健康检查的。//这个地方可以配置，不用写死

	registration.Check = check //检查

	//1.如何启动两个服务
	//2.即使我能够通过终端启动两个服务，但是注册到consul中的时候也会被覆盖。
	err = client.Agent().ServiceRegister(registration) //就是别人封装了一下
	if err != nil {
		panic(err)
	}
	go func() {
		err = server.Serve(lis) //这个是会阻塞后面的，所以要放在协程里。
		if err != nil {
			panic("failed to start grpc:" + err.Error())
		}
	}()
	//优雅退出，接收终止信号
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if err = client.Agent().ServiceDeregister(serviceID); err != nil {
		zap.S().Info("注销失败")
	}
	zap.S().Info("注销success")
}
