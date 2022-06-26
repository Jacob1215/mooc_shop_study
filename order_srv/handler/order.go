package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"math/rand"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"mxshop_srvs/order_srv/global"
	"mxshop_srvs/order_srv/model"
	"mxshop_srvs/order_srv/proto"
)

type OrderServer struct {
	proto.UnimplementedOrderServer
}

func (*OrderServer) CartItemList(ctx context.Context, req *proto.UserInfo) (*proto.CartItemListResponse, error) {
	//获取用户的购物车列表。
	var shopCarts []model.ShoppingCart //要拿到的列表。
	var rsp proto.CartItemListResponse
	if result := global.DB.Where(&model.ShoppingCart{User: req.Id}).Find(&shopCarts); result.Error != nil {
		return nil, result.Error
	} else {
		rsp.Total = int32(result.RowsAffected)
	}
	for _, shopCart := range shopCarts {
		rsp.Data = append(rsp.Data, &proto.ShopCartInfoResponse{
			Id:      shopCart.ID,
			UserId:  shopCart.User,
			GoodsId: shopCart.Goods,
			Nums:    shopCart.Nums,
			Checked: shopCart.Checked,
		})
	}
	return &rsp, nil
}

func (*OrderServer) CreateCartItem(ctx context.Context, req *proto.CartItemRequest) (*proto.ShopCartInfoResponse, error) {
	//将商品添加到购物车 1. 购物车中原本没有这件商品-新建一个记录 2.这个商品之前添加到了购物车-合并
	var shopCart model.ShoppingCart
	if result := global.DB.Where(&model.ShoppingCart{Goods: req.GoodsId, User: req.UserId}).First(&shopCart); result.RowsAffected == 1 {
		//如果记录已经存在，则合并购物车记录
		shopCart.Nums += req.Nums
	} else {
		//插入操作
		shopCart.User = req.UserId
		shopCart.Goods = req.GoodsId
		shopCart.Nums = req.Nums
		shopCart.Checked = false
	}
	global.DB.Save(&shopCart)
	return &proto.ShopCartInfoResponse{Id: shopCart.ID}, nil
}

func (*OrderServer) UpdateCartItem(ctx context.Context, req *proto.CartItemRequest) (*emptypb.Empty, error) {
	//商品数量//checked//更新购物车记录，更新数量和选中状态
	var shopCart model.ShoppingCart
	if result := global.DB.Where("goods=? and user=?", req.GoodsId, req.UserId).First(&shopCart); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "购物车记录不存在")
	}
	shopCart.Checked = req.Checked
	if req.Nums > 0 {
		shopCart.Nums = req.Nums
	}
	global.DB.Save(&shopCart)
	return &emptypb.Empty{}, nil
}

func (*OrderServer) DeleteCartItem(ctx context.Context, req *proto.CartItemRequest) (*emptypb.Empty, error) {
	//这里删除的时候要做验证是删除的该用户的商品，别删错了。//这里通过userId和goodsId来删除。
	if result := global.DB.Where("goods=? and user=?", req.GoodsId, req.UserId).Delete(&model.ShoppingCart{}); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "购物车记录不存在")
	}
	return &emptypb.Empty{}, nil
}

func (*OrderServer) OrderList(ctx context.Context, req *proto.OrderFilterRequest) (*proto.OrderListResponse, error) {
	var orders []model.OrderInfo
	var rsp proto.OrderListResponse
	var total int64
	//是后台管理系统查询 还是电商系统查询。
	global.DB.Where(&model.OrderInfo{User: req.UserId}).Count(&total) //gorm不会查询零值。所以这里放心大但写。
	rsp.Total = int32(total)
	//分页
	global.DB.Scopes(Paginate(int(req.Pages), int(req.PagePerNums))).Where(&model.OrderInfo{User: req.UserId}).Find(&orders)
	for _, order := range orders {
		rsp.Data = append(rsp.Data, &proto.OrderInfoResponse{
			Id:      order.ID,
			UserId:  order.User,
			OrderSn: order.OrderSn,
			PayType: order.PayType,
			Status:  order.Status,
			Post:    order.Post,
			Total:   order.OrderMount,
			Address: order.Address,
			Name:    order.SignerName,
			Mobile:  order.SignerMobile,
			AddTime: order.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &rsp, nil
}

func (*OrderServer) OrderDetail(ctx context.Context, req *proto.OrderRequest) (*proto.OrderInfoDetailResponse, error) {
	//先查询，
	var order model.OrderInfo
	var rsp proto.OrderInfoDetailResponse
	//这个订单的id是否是当前用户的订单，如果在web层用户传递过来一个id的订单。
	//web层应该先查询一下订单id是否是当前用户的。
	//这个在个人中心可以这样做，但是如果是后台管理系统。web层如果是后台管理系统，那么只传递order的id，如果是电商系统，还需要一个用户的id。
	if result := global.DB.Where(&model.OrderInfo{BaseModel: model.BaseModel{
		ID: req.Id}, User: req.UserId,
	}).First(&order); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "订单不存在")
	}
	orderInfo := proto.OrderInfoResponse{}
	orderInfo.Id = order.ID
	orderInfo.UserId = order.User
	orderInfo.PayType = order.PayType
	orderInfo.Status = order.Status
	orderInfo.Post = order.Post
	orderInfo.Total = order.OrderMount
	orderInfo.Address = order.Address
	orderInfo.Name = order.SignerName
	orderInfo.Mobile = order.SignerMobile
	rsp.Orderinfo = &orderInfo
	var orderGoods []model.OrderGoods
	global.DB.Where(&model.OrderGoods{Order: order.ID}).Find(&orderGoods)
	for _, orderGood := range orderGoods {
		rsp.Goods = append(rsp.Goods, &proto.OrderItemResponse{
			GoodsId:    orderGood.Goods,
			GoodsName:  orderGood.GoodsName,
			GoodsPrice: orderGood.GoodsPrice,
		})
	}
	return &rsp, nil
}

type OrderListener struct {
	Code        codes.Code //GRPC的code
	Detail      string
	ID          int32
	OrderAmount float32
	Ctx         context.Context
}

func (o *OrderListener) ExecuteLocalTransaction(msg *primitive.Message) primitive.LocalTransactionState {

	//商品的价格自己查询-访问商品服务（跨微服务）
	//库存的扣减-访问库存服务（跨微服务）
	//从购物车中获取到选中的商品。
	//订单的基本信息表-订单的商品信息表//真正的系统远不止这些功能。
	//从购物车中删除已购买的记录
	var orderInfo model.OrderInfo
	_ = json.Unmarshal(msg.Body, &orderInfo) //反向解析。
	//拿parentSpan
	parentSpan := opentracing.SpanFromContext(o.Ctx) //这样就可以直接拿到

	var goodsIds []int32
	var shopCarts []model.ShoppingCart
	goodsNumsMap := make(map[int32]int32)
	shopCartSpan := opentracing.GlobalTracer().StartSpan("select_shopcart", opentracing.ChildOf(parentSpan.Context()))

	if result := global.DB.Where(&model.ShoppingCart{User: orderInfo.User, Checked: true}).Find(&shopCarts); result.RowsAffected == 0 {
		o.Code = codes.InvalidArgument //屌啊，OrderListener的结构体，可以直接这样写。
		o.Detail = "没有选中结算的商品"
		return primitive.RollbackMessageState //没有商品，直接放心大但的回滚。刚才的消息不用发送出去了。
		//return nil, status.Errorf(codes.InvalidArgument, "没有选中结算的商品")
	}
	shopCartSpan.Finish()

	for _, shopCart := range shopCarts {
		goodsIds = append(goodsIds, shopCart.Goods)
		goodsNumsMap[shopCart.Goods] = shopCart.Nums
	}
	//跨服务调用。-gin之前的逻辑。//这个要去配置初始化第三方服务。
	queryGoodsSpan := opentracing.GlobalTracer().StartSpan("query_goods", opentracing.ChildOf(parentSpan.Context()))
	goods, err := global.GoodsSrvClient.BatchGetGoods(context.Background(), &proto.BatchGoodsIdInfo{
		Id: goodsIds,
	}) //获取商品的信息
	if err != nil {
		o.Code = codes.Internal
		o.Detail = "批量查询商品信息失败"
		return primitive.RollbackMessageState
		//return nil, status.Errorf(codes.Internal, "批量查询商品信息失败") //这个地方会报错，但是不知道为什么。
	}
	queryGoodsSpan.Finish()

	var orderAmount float32
	var orderGoods []*model.OrderGoods
	var goodsInvInfo []*proto.GoodsInvInfo
	for _, good := range goods.Data { //拿到商品的 信息并返回。
		orderAmount += good.ShopPrice * float32(goodsNumsMap[good.Id]) //这样不会丢失精度，不能简单转成int。会丢失。
		orderGoods = append(orderGoods, &model.OrderGoods{
			Goods:      good.Id,
			GoodsName:  good.Name,
			GoodsImage: good.GoodsFrontImage,
			GoodsPrice: good.ShopPrice,
			Nums:       goodsNumsMap[good.Id],
		})
		goodsInvInfo = append(goodsInvInfo, &proto.GoodsInvInfo{
			GoodsId: good.Id,
			Num:     goodsNumsMap[good.Id],
		})
	}
	//跨服务调用库存微服务进行库存扣减
	/*//1.调用库存服务的trysell
	2.调用仓库服务的trysell
	3.调用积分服务的tryAdd
	任何一个服务出现了异常，那么你得调用对应的所有的微服务的cancel接口
	如果所有的微服务都正常，那么你得调用所有的微服务的confirm
	*/
	queryInvSpan := opentracing.GlobalTracer().StartSpan("query_inv", opentracing.ChildOf(parentSpan.Context()))
	if _, err = global.InventorySrvClient.Sell(context.Background(), &proto.SellInfo{
		OrderSn:   orderInfo.OrderSn,
		GoodsInfo: goodsInvInfo,
	}); err != nil { //这个地方就自己完善一下。
		//如果是因为网络问题，这种如何避免误判。 大家自己改写一下sell的返回逻辑。
		//这是在已经扣减成功了，但是网络阻塞，得到err，如何避免误判，这时要去核对一下出现的错误是否是网络问题的。在这后面判断一下err码就行了。除此之外，全部commit。
		o.Code = codes.ResourceExhausted
		o.Detail = "库存扣减失败"
		return primitive.RollbackMessageState
		//return nil, status.Errorf(codes.ResourceExhausted, "扣减库存失败")
	}
	queryInvSpan.Finish()

	//生成订单表
	tx := global.DB.Begin() //在本地操作的时候开始事务，如果操作失败可以回滚。前面的只是查询，不会影响数据库。
	orderInfo.OrderMount = orderAmount
	saveOrderSpan := opentracing.GlobalTracer().StartSpan("save_order", opentracing.ChildOf(parentSpan.Context()))
	//order := model.OrderInfo{
	//	OrderSn:      GenerateOrderSn(orderInfo.User), //这个字段不能重新生成。
	//	OrderMount:   orderAmount,
	//	Address:      req.Address,
	//	SignerName:   req.Name,
	//	SignerMobile: req.Mobile,
	//	Post:         req.Post,
	//	User:         req.UserId,
	//}
	if result := tx.Save(&orderInfo); result.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "创建订单失败"
		return primitive.CommitMessageState //订单创建失败了，reback归还库存就要commit？//细节问题。
	}
	saveOrderSpan.Finish()
	//把外键加上//订单的ID和订单商品信息绑定上
	for _, orderGood := range orderGoods {
		orderGood.Order = orderInfo.ID
	}
	//批量插入orderGoods
	saveOrderGoodsSpan := opentracing.GlobalTracer().StartSpan("save_order_goods", opentracing.ChildOf(parentSpan.Context()))
	if result := tx.CreateInBatches(orderGoods, 100); result.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "批量创建订单失败"
		return primitive.CommitMessageState
		//return nil, status.Errorf(codes.Internal, "创建订单失败")
	} //插入订单表。
	saveOrderGoodsSpan.Finish()
	//删除已购买的商品信息。
	deleteShopCartSpan := opentracing.GlobalTracer().StartSpan("delete_shopcart", opentracing.ChildOf(parentSpan.Context()))
	if result := tx.Where(&model.ShoppingCart{User: orderInfo.User, Checked: true}).Delete(&model.ShoppingCart{}); result.RowsAffected == 0 {
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "删除购物车记录失败"
		return primitive.CommitMessageState
	}
	deleteShopCartSpan.Finish()
	//发送延时消息
	newProducer, err := rocketmq.NewProducer(producer.WithNameServer([]string{"192.168.1.104:9876"}))
	if err != nil {
		panic("生成producer失败")
	}
	if err := newProducer.Start(); err != nil {
		panic("启动producer失败")
	}
	msg2 := primitive.NewMessage("order_timeout", msg.Body)
	msg2.WithDelayTimeLevel(5) //level 5。1分钟。
	//发送消息
	_, err = newProducer.SendSync(context.Background(), msg)
	if err != nil {
		zap.S().Errorf("发送延时消息失败: %v\n", err)
		tx.Rollback()
		o.Code = codes.Internal
		o.Detail = "发送延时消息失败"
		return primitive.CommitMessageState
	}
	//if err = newProducer.Shutdown(); err != nil {
	//	panic("关闭producer失败")
	//}
	//提交事务
	tx.Commit()

	//本地执行逻辑无缘无故失败，代码异常 宕机
	o.Code = codes.OK
	return primitive.RollbackMessageState //本地事务成功了，reback消息就不做了。
}

// When no response to prepare(half) message. broker will send check message to check the transaction status, and this
// method will be invoked to get local transaction status.
func (o *OrderListener) CheckLocalTransaction(msg *primitive.MessageExt) primitive.LocalTransactionState {
	var orderInfo model.OrderInfo
	_ = json.Unmarshal(msg.Body, &orderInfo)
	//怎么检查之前的逻辑是否完成。
	if result := global.DB.Where(model.OrderInfo{OrderSn: orderInfo.OrderSn}).First(&orderInfo); result.RowsAffected == 0 {
		return primitive.CommitMessageState //本地事务执行失败了，就把这个消息发送出去。你并不能说明这里就是库存已经扣减了。
		//所以要去库存服务做一个幂等性的保证。
	}
	return primitive.CommitMessageState
}

func (*OrderServer) CreateOrder(ctx context.Context, req *proto.OrderRequest) (*proto.OrderInfoResponse, error) {
	//新建订单之前，准备一个半消息。

	orderListener := OrderListener{Ctx: ctx} //这个别忘了传。
	p, err := rocketmq.NewTransactionProducer(
		&orderListener, //这里要返回地址
		producer.WithNameServer([]string{"192.168.1.104:9876"}))
	if err != nil {
		zap.S().Errorf("生成producer失败:%s", err.Error())
		return nil, err
	}
	if err = p.Start(); err != nil {
		zap.S().Errorf("启动producer失败:%s", err.Error())
		return nil, err
	}

	order := model.OrderInfo{
		OrderSn:      GenerateOrderSn(req.UserId), //拿到订单号//这个订单号非常重要，会发送到半消息里面。
		Address:      req.Address,
		SignerName:   req.Name,
		SignerMobile: req.Mobile,
		Post:         req.Post,
		User:         req.UserId,
	}
	//应该在消息中具体指明一个订单的具体的商品的扣减情况。

	jsonString, _ := json.Marshal(order)                                                                        // 要转换成string。
	_, err = p.SendMessageInTransaction(context.Background(), primitive.NewMessage("order_reback", jsonString)) //这条消息是发送需要归还库存的消息。没有commit，就表示新建订单成功。
	if err != nil {
		fmt.Printf("发送失败：%s\n", err)
		return nil, status.Errorf(codes.Internal, "发送order消息失败")
	}
	if orderListener.Code != codes.OK { //这个逻辑是库存不够，调用归还库存的逻辑，这里commit表示新建订单失败，commit之前发送的reback的逻辑。用以归还库存
		return nil, status.Error(orderListener.Code, orderListener.Detail)
	}
	//time.Sleep(time.Hour) //为什么要设置sleep，就是为了测试回查逻辑。

	//if err = p.Shutdown(); err != nil {
	//	panic("关闭transaction失败")
	//}

	return &proto.OrderInfoResponse{
		Id:      orderListener.ID,
		OrderSn: order.OrderSn, //因为这个生成的单号不能变，所以这里还是order的单号。
		Total:   orderListener.OrderAmount,
	}, nil
}
func GenerateOrderSn(userId int32) string { //订单号的生成规则
	//年月日时分秒。并发很高，可能导致是一样的，再加上用户id+2位随机数
	now := time.Now()
	rand.Seed(time.Now().UnixNano()) //UnixNano returns t as a Unix time, the number of nanoseconds elapsed
	// since January 1, 1970 UTC.
	orderSn := fmt.Sprintf("%d%d%d%d%d%d%d%d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Nanosecond(),
		userId, rand.Intn(90)+10)
	return orderSn
}

func (*OrderServer) UpdateOrderStatus(ctx context.Context, req *proto.OrderStatus) (*emptypb.Empty, error) {
	//先查询，再更新，有两条语句。实际上有两条sql执行，select和update语句。
	if result := global.DB.Model(&model.OrderInfo{}).Where("order_sn = ?", req.OrderSn).Update("status", req.Status); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "订单不存在")
	}
	return &emptypb.Empty{}, nil
}

func OrderTimeout(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for i := range msgs {
		var orderInfo model.OrderInfo
		_ = json.Unmarshal(msgs[i].Body, &orderInfo)
		fmt.Printf("获取到订单超时消息:%v\n", time.Now())
		//查询订单的支付状态，如果已支付什么都不做，如果未支付，归还库存。
		var order model.OrderInfo
		if result := global.DB.Model(model.OrderInfo{}).Where(model.OrderInfo{OrderSn: orderInfo.OrderSn}).First(&order); result.RowsAffected == 0 {
			return consumer.ConsumeSuccess, nil
		} //这句话是查询有没有订单信息。
		if order.Status != "TRADE_SUCCESS" {
			tx := global.DB.Begin()
			order.Status = "TRADE_CLOSED"
			tx.Save(&order)
			//归还库存，我们可以模仿order中发送一个消息到 order_reback中去。
			newProducer, err := rocketmq.NewProducer(producer.WithNameServer([]string{"192.168.1.104:9876"}))
			if err != nil {
				panic("生成producer失败")
			}
			if err = newProducer.Start(); err != nil {
				panic("启动producer失败")
			}
			_, err = newProducer.SendSync(context.Background(), primitive.NewMessage("order_reback", msgs[i].Body))
			if err != nil {
				tx.Rollback()
				fmt.Printf("发送失败：%s\n", err)
				return consumer.ConsumeRetryLater, nil
			}
			//if err = newProducer.Shutdown(); err != nil {
			//	panic("关闭producer失败")
			//}
			//修改订单的状态为已支付。
		}
	}
	return consumer.ConsumeSuccess, nil
}
