package main

import (
	"fmt"
	goredislib "github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"sync"
	"time"
)

func main() {
	// Create a pool with go-redis (or redigo) which is the pool redisync will
	// use while communicating with Redis. This can also be any pool that
	// implements the `redis.Pool` interface.
	client := goredislib.NewClient(&goredislib.Options{
		Addr: "192.168.1.114:6379",
	})
	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)
	// Create an instance of redisync to be used to obtain a mutual exclusion
	// lock.
	rs := redsync.New(pool)
	// Obtain a new mutex by using the same name for all instances wanting the
	// same lock.
	gNum := 2
	mutexname := "my-global-mutex" //用法，锁要命名。
	var wg sync.WaitGroup
	wg.Add(gNum)
	for i := 0; i < gNum; i++ {
		go func() {
			defer wg.Done()
			mutex := rs.NewMutex(mutexname)
			fmt.Println("开始获取锁")
			// Obtain a lock for our given mutex. After this is successful, no one else
			// can obtain the same lock (the same mutex name) until we unlock it.
			if err := mutex.Lock(); err != nil {
				panic(err)
			}
			fmt.Println("获取锁成功")
			time.Sleep(time.Second * 5)
			fmt.Println("开始释放锁")
			// Do your work that requires the lock.
			// Release the lock so other processes or threads can obtain a lock.
			if ok, err := mutex.Unlock(); !ok || err != nil {
				panic("unlock failed")
			}
			fmt.Println("释放锁成功")
		}()
	}
	wg.Wait() //防止主协程退出。
}
