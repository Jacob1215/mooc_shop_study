package main

import (
	"context"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"time"
)

type OrderListener struct {
}

func (o *OrderListener) ExecuteLocalTransaction(addr *primitive.Message) primitive.LocalTransactionState {
	fmt.Println("开始执行本地逻辑")
	time.Sleep(time.Second * 3)
	fmt.Println("执行本地逻辑失败")

	//本地执行逻辑无缘无故失败，代码异常 宕机
	return primitive.UnknowState
}

// When no response to prepare(half) message. broker will send check message to check the transaction status, and this
// method will be invoked to get local transaction status.
func (o *OrderListener) CheckLocalTransaction(ext *primitive.MessageExt) primitive.LocalTransactionState {
	fmt.Println("rocketmq的消息回查")
	time.Sleep(time.Second * 15)
	return primitive.CommitMessageState
}
func main() {
	p, err := rocketmq.NewTransactionProducer(
		&OrderListener{}, //这里要返回地址
		producer.WithNameServer([]string{"192.168.1.104:9876"}))
	if err != nil {
		panic("生成producer失败")
	}
	if err = p.Start(); err != nil {
		panic("启动producer失败")
	}
	res, err := p.SendMessageInTransaction(context.Background(), primitive.NewMessage("transTopic", []byte("this is transaction message3-unknownstate")))
	if err != nil {
		fmt.Printf("发送失败：%s\n", err)
	} else {
		fmt.Printf("发送成功:%s\n", res.String())
	}
	time.Sleep(time.Hour) //为什么要设置sleep，就是为了测试回查逻辑。

	if err = p.Shutdown(); err != nil {
		panic("关闭transaction失败")
	}
}
