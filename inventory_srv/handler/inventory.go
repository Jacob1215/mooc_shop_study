package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	goredislib "github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"mxshop_srvs/goods_srv/global"
	"mxshop_srvs/inventory_srv/model"
	"mxshop_srvs/inventory_srv/proto"
	"time"
)

type InventoryServer struct {
	proto.UnimplementedInventoryServer
}

func (*InventoryServer) SetInv(ctx context.Context, req *proto.GoodsInvInfo) (*emptypb.Empty, error) {
	//设置库存，如果我要更新库存。没有就设置，有就更新。比较简单
	var inv model.Inventory
	global.DB.Where(&model.Inventory{Goods: req.GoodsId}).First(&inv) //指定查找的goods是唯一的，
	inv.Goods = req.GoodsId
	inv.Stocks = req.Num
	global.DB.Save(&inv)
	return &emptypb.Empty{}, nil
}

func (*InventoryServer) InvDetail(ctx context.Context, req *proto.GoodsInvInfo) (*proto.GoodsInvInfo, error) {
	var inv model.InventoryNew
	if result := global.DB.Where(&model.Inventory{Goods: req.GoodsId}).First(&inv); result.RowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "invdetail库存信息不存在")
	}
	return &proto.GoodsInvInfo{
		GoodsId: inv.Goods,
		Num:     inv.Stocks - inv.Freeze, //减去已经冻结的字段。
	}, nil
}

//var m sync.Mutex //声明全局锁
func (*InventoryServer) Sell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//扣减库存， 本地事务【1：10，2：5，3：20】
	//数据库基本的一个应用场景：数据库事务。同时成功，同时失败
	//gorm是支持事务的
	//并发情况之下，可能会出现超卖。
	client := goredislib.NewClient(&goredislib.Options{ //这个初始化工作放到全局去做。
		Addr: "192.168.1.114:6379", //忘记redis在哪个端口了。
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	rs := redsync.New(pool)
	//把selldetail声明好。保存扣减商品的信息，主要是OrderSn
	//这个时候应该先查询表，然后确定这个订单是否已经扣减过库存了，已经扣减过了就别扣减了。
	//并发的时候会有漏洞，比如同时查到没有扣减，就同时做了扣减。
	//这个是select+insert机制。
	sellDetail := model.StockSellDetail{
		OrderSn: req.OrderSn,
		Status:  1,
	}
	var details []model.GoodsDetail

	tx := global.DB.Begin() //开启事务
	for _, goodInfo := range req.GoodsInfo {
		details = append(details, model.GoodsDetail{
			Goods: goodInfo.GoodsId,
			Num:   goodInfo.Num,
		})
		//m.Lock() //获取锁。这把锁会有性能问题。
		mutex := rs.NewMutex(fmt.Sprintf("goods_%d", goodInfo.GoodsId))
		if err := mutex.Lock(); err != nil {
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		fmt.Println("获取锁成功")
		time.Sleep(time.Second * 8)
		fmt.Println("开始释放锁")
		// Do your work that requires the lock.
		// Release the lock so other processes or threads can obtain a lock.
		if ok, err := mutex.Unlock(); !ok || err != nil {
			panic("unlock failed")
		}
		fmt.Println("释放锁成功")
		var inv model.Inventory
		//for { //不停尝试，不能放弃。
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚，事务开始后到这条的操作全部不要了
			return nil, status.Errorf(codes.InvalidArgument, "sell，没有库存信息")
		}
		//判断库存是否充足
		if inv.Stocks < goodInfo.Num {
			tx.Rollback()
			return nil, status.Errorf(codes.ResourceExhausted, "sell，库存不足")
		}
		//扣减， 会出现数据不一致的问题， -锁，分布式锁，专门解决数据不一致的问题。
		inv.Stocks -= goodInfo.Num
		tx.Save(&inv)
		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
		//update inventory set stocks = stocks - 1,version = version+1 where goods =goodsid and version = version
		//这种写法有瑕疵。非常不明显，
		//零值 对于int类型来说，默认值是0 这种会被gorm给忽略掉。
		//if result := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where(
		//	"goods = ? and version = ?", goodInfo.GoodsId, inv.Version).Updates(
		//	model.Inventory{Stocks: inv.Stocks, Version: inv.Version + 1}); result.RowsAffected == 0 {
		//	zap.S().Info("库存扣减失败")
		//} else {
		//	break //成功就把for循环break掉。
		//}
		//}
		//tx.Save(&inv) //这里必须用事务的tx
	}
	sellDetail.Detail = details
	//写selldetail表
	if result := tx.Create(&sellDetail); result.RowsAffected == 0 {
		tx.Rollback()
		return nil, status.Errorf(codes.Internal, "保存库存扣减List失败。")
	}
	tx.Commit() //需要自己手动提交操作。
	//m.Unlock()  //释放锁。go语言自己提供的锁。必须等到数据库事务提交之后才unlock
	return &emptypb.Empty{}, nil
}
func (*InventoryServer) Reback(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//库存归还：1：订单超时归还，很多订单下了不买。2、订单创建失败。归还之前扣减的库存。3、手动归还。
	tx := global.DB.Begin() //开启事务

	for _, goodInfo := range req.GoodsInfo {
		var inv model.Inventory
		if result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚，事务开始后到这条的操作全部不要了
			return nil, status.Errorf(codes.InvalidArgument, "sell，没有库存信息")
		}
		//归还， 会出现数据不一致的问题， -锁，分布式锁，专门解决数据不一致的问题。
		inv.Stocks += goodInfo.Num
		tx.Save(&inv) //这里必须用事务的tx
	}
	tx.Commit() //需要自己手动提交操作。
	return &emptypb.Empty{}, nil
}

func (*InventoryServer) TrySell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//扣减库存， 本地事务【1：10，2：5，3：20】
	//数据库基本的一个应用场景：数据库事务。同时成功，同时失败
	//gorm是支持事务的
	//并发情况之下，可能会出现超卖。
	client := goredislib.NewClient(&goredislib.Options{ //这个初始化工作放到全局去做。
		Addr: "192.168.1.114:6379", //忘记redis在哪个端口了。
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	rs := redsync.New(pool)
	tx := global.DB.Begin() //开启事务
	for _, goodInfo := range req.GoodsInfo {
		//m.Lock() //获取锁。这把锁会有性能问题。
		mutex := rs.NewMutex(fmt.Sprintf("goods_%d", goodInfo.GoodsId))
		if err := mutex.Lock(); err != nil {
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		fmt.Println("获取锁成功")
		time.Sleep(time.Second * 8)
		fmt.Println("开始释放锁")
		// Do your work that requires the lock.
		// Release the lock so other processes or threads can obtain a lock.
		if ok, err := mutex.Unlock(); !ok || err != nil {
			panic("unlock failed")
		}
		fmt.Println("释放锁成功")
		var inv model.InventoryNew
		//for { //不停尝试，不能放弃。
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚，事务开始后到这条的操作全部不要了
			return nil, status.Errorf(codes.InvalidArgument, "sell，没有库存信息")
		}
		//判断库存是否充足
		if inv.Stocks < goodInfo.Num {
			tx.Rollback()
			return nil, status.Errorf(codes.ResourceExhausted, "sell，库存不足")
		}
		//扣减， 会出现数据不一致的问题， -锁，分布式锁，专门解决数据不一致的问题。
		//inv.Stocks -= goodInfo.Num
		inv.Freeze += goodInfo.Num
		tx.Save(&inv)
		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
		//update inventory set stocks = stocks - 1,version = version+1 where goods =goodsid and version = version
		//这种写法有瑕疵。非常不明显，
		//零值 对于int类型来说，默认值是0 这种会被gorm给忽略掉。
		//if result := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where(
		//	"goods = ? and version = ?", goodInfo.GoodsId, inv.Version).Updates(
		//	model.Inventory{Stocks: inv.Stocks, Version: inv.Version + 1}); result.RowsAffected == 0 {
		//	zap.S().Info("库存扣减失败")
		//} else {
		//	break //成功就把for循环break掉。
		//}
		//}
		//tx.Save(&inv) //这里必须用事务的tx
	}
	tx.Commit() //需要自己手动提交操作。
	//m.Unlock()  //释放锁。go语言自己提供的锁。必须等到数据库事务提交之后才unlock
	return &emptypb.Empty{}, nil
}

func (*InventoryServer) ComfirmSell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//扣减库存， 本地事务【1：10，2：5，3：20】
	//数据库基本的一个应用场景：数据库事务。同时成功，同时失败
	//gorm是支持事务的
	//并发情况之下，可能会出现超卖。
	client := goredislib.NewClient(&goredislib.Options{ //这个初始化工作放到全局去做。
		Addr: "192.168.1.114:6379", //忘记redis在哪个端口了。
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	rs := redsync.New(pool)
	tx := global.DB.Begin() //开启事务
	for _, goodInfo := range req.GoodsInfo {
		//m.Lock() //获取锁。这把锁会有性能问题。
		mutex := rs.NewMutex(fmt.Sprintf("goods_%d", goodInfo.GoodsId))
		if err := mutex.Lock(); err != nil {
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		fmt.Println("获取锁成功")
		time.Sleep(time.Second * 8)
		fmt.Println("开始释放锁")
		// Do your work that requires the lock.
		// Release the lock so other processes or threads can obtain a lock.
		if ok, err := mutex.Unlock(); !ok || err != nil {
			panic("unlock failed")
		}
		fmt.Println("释放锁成功")
		var inv model.InventoryNew
		//for { //不停尝试，不能放弃。
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚，事务开始后到这条的操作全部不要了
			return nil, status.Errorf(codes.InvalidArgument, "sell，没有库存信息")
		}
		//判断库存是否充足
		if inv.Stocks < goodInfo.Num {
			tx.Rollback()
			return nil, status.Errorf(codes.ResourceExhausted, "sell，库存不足")
		}
		//扣减， 会出现数据不一致的问题， -锁，分布式锁，专门解决数据不一致的问题。
		//inv.Stocks -= goodInfo.Num
		inv.Freeze -= goodInfo.Num //确认完成之后，冻结的字段需要减下来。
		inv.Stocks -= goodInfo.Num //确认完成之后，就真实的扣减掉了。
		tx.Save(&inv)
		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
		//update inventory set stocks = stocks - 1,version = version+1 where goods =goodsid and version = version
		//这种写法有瑕疵。非常不明显，
		//零值 对于int类型来说，默认值是0 这种会被gorm给忽略掉。
		//if result := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where(
		//	"goods = ? and version = ?", goodInfo.GoodsId, inv.Version).Updates(
		//	model.Inventory{Stocks: inv.Stocks, Version: inv.Version + 1}); result.RowsAffected == 0 {
		//	zap.S().Info("库存扣减失败")
		//} else {
		//	break //成功就把for循环break掉。
		//}
		//}
		//tx.Save(&inv) //这里必须用事务的tx
	}
	tx.Commit() //需要自己手动提交操作。
	//m.Unlock()  //释放锁。go语言自己提供的锁。必须等到数据库事务提交之后才unlock
	return &emptypb.Empty{}, nil
}

func (*InventoryServer) CancelSell(ctx context.Context, req *proto.SellInfo) (*emptypb.Empty, error) {
	//扣减库存， 本地事务【1：10，2：5，3：20】
	//数据库基本的一个应用场景：数据库事务。同时成功，同时失败
	//gorm是支持事务的
	//并发情况之下，可能会出现超卖。
	client := goredislib.NewClient(&goredislib.Options{ //这个初始化工作放到全局去做。
		Addr: "192.168.1.114:6379", //忘记redis在哪个端口了。
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	rs := redsync.New(pool)
	tx := global.DB.Begin() //开启事务
	for _, goodInfo := range req.GoodsInfo {
		//m.Lock() //获取锁。这把锁会有性能问题。
		mutex := rs.NewMutex(fmt.Sprintf("goods_%d", goodInfo.GoodsId))
		if err := mutex.Lock(); err != nil {
			return nil, status.Errorf(codes.Internal, "获取redis分布式锁异常")
		}
		fmt.Println("获取锁成功")
		time.Sleep(time.Second * 8)
		fmt.Println("开始释放锁")
		// Do your work that requires the lock.
		// Release the lock so other processes or threads can obtain a lock.
		if ok, err := mutex.Unlock(); !ok || err != nil {
			panic("unlock failed")
		}
		fmt.Println("释放锁成功")
		var inv model.InventoryNew
		//for { //不停尝试，不能放弃。
		if result := global.DB.Where(&model.Inventory{Goods: goodInfo.GoodsId}).First(&inv); result.RowsAffected == 0 {
			tx.Rollback() //回滚，事务开始后到这条的操作全部不要了
			return nil, status.Errorf(codes.InvalidArgument, "sell，没有库存信息")
		}
		//判断库存是否充足
		if inv.Stocks < goodInfo.Num {
			tx.Rollback()
			return nil, status.Errorf(codes.ResourceExhausted, "sell，库存不足")
		}
		//扣减， 会出现数据不一致的问题， -锁，分布式锁，专门解决数据不一致的问题。
		//inv.Stocks -= goodInfo.Num
		inv.Freeze -= goodInfo.Num //已经失败了，扣减下来就行了，不用改stock。
		tx.Save(&inv)
		if ok, err := mutex.Unlock(); !ok || err != nil {
			return nil, status.Errorf(codes.Internal, "释放redis分布式锁异常")
		}
		//update inventory set stocks = stocks - 1,version = version+1 where goods =goodsid and version = version
		//这种写法有瑕疵。非常不明显，
		//零值 对于int类型来说，默认值是0 这种会被gorm给忽略掉。
		//if result := tx.Model(&model.Inventory{}).Select("Stocks", "Version").Where(
		//	"goods = ? and version = ?", goodInfo.GoodsId, inv.Version).Updates(
		//	model.Inventory{Stocks: inv.Stocks, Version: inv.Version + 1}); result.RowsAffected == 0 {
		//	zap.S().Info("库存扣减失败")
		//} else {
		//	break //成功就把for循环break掉。
		//}
		//}
		//tx.Save(&inv) //这里必须用事务的tx
	}
	tx.Commit() //需要自己手动提交操作。
	//m.Unlock()  //释放锁。go语言自己提供的锁。必须等到数据库事务提交之后才unlock
	return &emptypb.Empty{}, nil
}

//自动归还库存
func AutoReback(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	type OrderInfo struct { //临时定义一个，只供自己使用。
		OrderSn string
	}
	for i := range msgs {
		//既然是归还库存，那么我应该具体的知道每件商品应该归还多少。但是有一个问题是什么？重复归还的问题。
		//这个接口应该确保幂等性。你不能因为消息的重复发送导致一个订单的库存归还多次，还有没有扣减的库存你别归还。
		//如何确保这些都没有问题，新建一张表，这张表记录了详细的订单扣减细节，以及归还细节。
		var orderInfo OrderInfo
		if err := json.Unmarshal(msgs[i].Body, &orderInfo); err != nil {
			zap.S().Errorf("解析json失败：%v\n", msgs[i].Body)
			return consumer.ConsumeSuccess, nil //这个返回啥自己定义。
		}
		//去将inv的库存加回去，将selldetail的status设置为2，要在事务中进行。
		tx := global.DB.Begin()
		var sellDetail model.StockSellDetail
		if result := tx.Model(&model.StockSellDetail{}).Where(&model.StockSellDetail{OrderSn: orderInfo.OrderSn, Status: 1}).First(&sellDetail); result.RowsAffected == 0 {
			return consumer.ConsumeSuccess, nil //如果没有查询到需要归还的东西。
		}
		//如果查询到，则逐个归还库存。
		for _, orderGood := range sellDetail.Detail {
			//update怎么用
			//update语句的 update xx set stocks= stocks +2
			if result := tx.Model(&model.Inventory{}).Where(&model.Inventory{Goods: orderGood.Goods}).Update("stocks",
				gorm.Expr("stocks+?", orderGood.Num)); result.RowsAffected == 0 { //这样避免了去查询数据库里面有多少个stocks。
				tx.Rollback()
				return consumer.ConsumeRetryLater, nil //归还商品
			}
		}
		sellDetail.Status = 2
		if result := tx.Model(&model.StockSellDetail{}).Where(&model.StockSellDetail{OrderSn: orderInfo.OrderSn}).Update("status", 2); result.RowsAffected == 0 {
			tx.Rollback()
			return consumer.ConsumeRetryLater, nil
		}
		tx.Commit()
		return consumer.ConsumeSuccess, nil
	}
	return consumer.ConsumeSuccess, nil
}
