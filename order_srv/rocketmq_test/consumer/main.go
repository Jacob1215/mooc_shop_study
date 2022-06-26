package main

import (
	"context"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"time"
)

func main() {
	pushConsumer, err := rocketmq.NewPushConsumer(
		consumer.WithNameServer([]string{"192.168.1.104:9876"}),
		consumer.WithGroupName("mxshop"), //只要groupname一致，多个生产者的信息就可以同时发送到一个rmq供多个消费者使用。
	)
	if err != nil {
		panic("新建消费者失败")
	}
	err = pushConsumer.Subscribe("imooc1", consumer.MessageSelector{}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for i := range msgs {
			fmt.Printf("获取得到的值：%v\n", msgs[i])
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		fmt.Println("读取消息失败。")
	}
	err = pushConsumer.Start()
	if err != nil {
		panic("开始失败")
	}
	//不能让主goroutine退出
	time.Sleep(time.Hour)
	_ = pushConsumer.Shutdown()
}
