package service

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/garyburd/redigo/redis"
)

//initRedisPool redis初始化
func initRedisPool(redisConf RedisConf) (pool *redis.Pool, err error) {
	pool = &redis.Pool{
		MaxIdle:     redisConf.RedisMaxIdle,
		MaxActive:   redisConf.RedisMaxActive,
		IdleTimeout: time.Duration(redisConf.RedisIdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisConf.RedisAddr, redis.DialPassword("123456"))
		},
	}

	conn := pool.Get()
	defer conn.Close()

	_, err = conn.Do("ping")
	if err != nil {
		logs.Error("ping redis failed, err:%v", err)
		return
	}
	return
}

//initRedis 初始化两个RedisPool
func initRedis(conf *SecLayerConf) (err error) {
	secLayerContext.proxy2LayerRedisPool, err = initRedisPool(conf.Proxy2LayerRedis)
	if err != nil {
		logs.Error("init proxy2layer redis pool failed, err:%v", err)
		return
	}

	secLayerContext.layer2ProxyRedisPool, err = initRedisPool(conf.Layer2ProxyRedis)
	if err != nil {
		logs.Error("init layer2proxy redis pool failed, err:%v", err)
		return
	}

	return
}

//RunProcess 运行处理线程
func RunProcess() (err error) {
	for i := 0; i < secLayerContext.secLayerConf.ReadGoroutineNum; i++ {
		secLayerContext.waitGroup.Add(1)
		go HandleReader()
	}

	for i := 0; i < secLayerContext.secLayerConf.WriteGoroutineNum; i++ {
		secLayerContext.waitGroup.Add(1)
		go HandleWrite()
	}

	for i := 0; i < secLayerContext.secLayerConf.HandleUserGoroutineNum; i++ {
		secLayerContext.waitGroup.Add(1)
		go HandleUser()
	}

	logs.Debug("all process goroutine started")
	secLayerContext.waitGroup.Wait()
	logs.Debug("wait all goroutine exited")
	return
}

//HandleReader 从redis队列中读取
func HandleReader() {
	logs.Debug("read goroutine running")
	for {
		conn := secLayerContext.proxy2LayerRedisPool.Get()
		for {
			ret, err := conn.Do("blpop", secLayerContext.secLayerConf.Proxy2LayerRedis.RedisQueueName, 0)
			if err != nil {
				logs.Error("pop from queue failed, err:%v", err)
				break
			}

			tmp, ok := ret.([]interface{})
			if !ok || len(tmp) != 2 {
				logs.Error("pop from queue failed, err:%v", err)
				continue
			}

			data, ok := tmp[1].([]byte)
			if !ok {
				logs.Error("pop from queue failed, err:%v", err)
				continue
			}
			logs.Debug("pop from queue, data:%s", string(data))

			var req SecRequest
			err = json.Unmarshal([]byte(data), &req)
			if err != nil {
				logs.Error("unmarshal to secrequest failed, err:%v", err)
				continue
			}

			now := time.Now().Unix()
			if now-req.AccessTime.Unix() >= int64(secLayerContext.secLayerConf.MaxRequestWaitTimeout) {
				logs.Warn("req[%v] is expire", req)
				continue
			}

			timer := time.NewTicker(time.Millisecond * time.Duration(secLayerContext.secLayerConf.SendToHandleChanTimeout))

			select {
			case secLayerContext.Read2HandleChan <- &req:
			case <-timer.C:
				logs.Warn("send to handle chan timeout, req:%v", req)
				break
			}
		}

		conn.Close()
	}
}

//HandleWrite res发送给redis
func HandleWrite() {
	logs.Debug("handle write running")
	for res := range secLayerContext.Handle2WriteChan {
		err := sendToRedis(res)
		if err != nil {
			logs.Error("send to redis, err:%v, res:%v", err, res)
			continue
		}
	}
}

//sendToRedis 把res加入到redis队列中
func sendToRedis(res *SecResponse) (err error) {
	data, err := json.Marshal(res)
	if err != nil {
		logs.Error("marshal failed, err:%v", err)
		return
	}

	conn := secLayerContext.layer2ProxyRedisPool.Get()
	_, err = conn.Do("rpush", secLayerContext.secLayerConf.Layer2ProxyRedis.RedisQueueName, string(data))
	if err != nil {
		logs.Warn("rpush to redis failed, err:%v", err)
		return
	}

	return
}

//HandleUser 处理用户相关
func HandleUser() {
	logs.Debug("handle user running")
	for req := range secLayerContext.Read2HandleChan {
		logs.Debug("begin process request:%v", req)
		res, err := HandleSecKill(req)
		if err != nil {
			logs.Warn("process request %v failed, err:%v", err)
			res = &SecResponse{
				Code: ErrServiceBusy,
			}
		}

		timer := time.NewTicker(time.Millisecond * time.Duration(secLayerContext.secLayerConf.SendToWriteChanTimeout))
		select {
		case secLayerContext.Handle2WriteChan <- res:
		case <-timer.C:
			logs.Warn("send to response chan timeout, res:%v", res)
			break
		}

	}
	return
}

//HandleSecKill 处理秒杀的相关功能
func HandleSecKill(req *SecRequest) (res *SecResponse, err error) {
	secLayerContext.RWSecProductLock.RLock()
	defer secLayerContext.RWSecProductLock.RUnlock()

	res = &SecResponse{}
	res.UserId = req.UserId
	res.ProductID = req.ProductID
	product, ok := secLayerContext.secLayerConf.SecProductInfoMap[req.ProductID]
	if !ok {
		logs.Error("not found product:%v", req.ProductID)
		res.Code = ErrNotFoundProduct
		return
	}
	if product.Status == ProductStatusSoldout {
		res.Code = ErrSoldout
		return
	}

	//限速
	now := time.Now().Unix()
	alreadySoldCount := product.secLimit.Check(now)
	if alreadySoldCount >= product.SoldMaxLimit {
		res.Code = ErrRetry
		return
	}

	//一个用户一个产品的购买量
	secLayerContext.HistoryMapLock.Lock()
	userHistory, ok := secLayerContext.HistoryMap[req.UserId]
	if !ok {
		userHistory = &UserBuyHistory{
			history: make(map[int]int, 16),
		}

		secLayerContext.HistoryMap[req.UserId] = userHistory
	}
	histryCount := userHistory.GetProductBuyCount(req.ProductID)
	secLayerContext.HistoryMapLock.Unlock()

	if histryCount >= product.OnePersonBuyLimit {
		res.Code = ErrAlreadyBuy
		return
	}

	//一个产品总共有多少个
	curSoldCount := secLayerContext.productCountMgr.Count(req.ProductID)
	if curSoldCount >= product.Total {
		res.Code = ErrSoldout
		product.Status = ProductStatusSoldout
		return
	}

	//概率
	curRate := rand.Float64()
	fmt.Printf("curRate:%v product:%v count:%v total:%v\n", curRate, product.BuyRate, curSoldCount, product.Total)
	if curRate > product.BuyRate {
		res.Code = ErrRetry
		return
	}
	userHistory.Add(req.ProductID, 1)
	secLayerContext.productCountMgr.Add(req.ProductID, 1)

	//用户id&商品id&当前时间&密钥
	res.Code = ErrSecKillSucc
	tokenData := fmt.Sprintf("userId=%d&ProductID=%d&timestamp=%d&security=%s",
		req.UserId, req.ProductID, now, secLayerContext.secLayerConf.TokenPasswd)

	res.Token = fmt.Sprintf("%x", md5.Sum([]byte(tokenData)))
	res.TokenTime = now

	return
}
