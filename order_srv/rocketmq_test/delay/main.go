package main

import (
	"context"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

func main() {
	newProducer, err := rocketmq.NewProducer(producer.WithNameServer([]string{"192.168.1.104:9876"}))
	if err != nil {
		panic("生成producer失败")
	}
	if err := newProducer.Start(); err != nil {
		panic("启动producer失败")
	}
	msg := primitive.NewMessage("imooc1", []byte("thie is delay message"))
	msg.WithDelayTimeLevel(2)
	//发送消息
	res, err := newProducer.SendSync(context.Background(), msg)
	if err != nil {
		fmt.Printf("发送失败：%s\n", err)
	} else {
		fmt.Printf("发送成功:%s\n", res.String())
	}
	if err = newProducer.Shutdown(); err != nil {
		panic("关闭producer失败")
	}

}
