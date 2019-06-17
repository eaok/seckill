package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/garyburd/redigo/redis"
)

//WriteHandle 把SecRequest写入到redis队列
func WriteHandle() {
	for {
		req := <-secKillConf.SecReqChan
		conn := secKillConf.proxy2LayerRedisPool.Get()

		data, err := json.Marshal(req)
		if err != nil {
			logs.Error("json.Marshal failed, error:%v req:%v", err, req)
			conn.Close()
			continue
		}

		_, err = conn.Do("LPUSH", "sec_queue", string(data))
		if err != nil {
			logs.Error("lpush failed, err:%v, req:%v", err, req)
			conn.Close()
			continue
		}

		conn.Close()
	}
}

//ReadHandle 从redis队列中读取
func ReadHandle() {
	for {
		conn := secKillConf.proxy2LayerRedisPool.Get()
		reply, err := conn.Do("RPOP", "recv_queue")
		data, err := redis.String(reply, err)
		if err == redis.ErrNil {
			time.Sleep(time.Second)
			conn.Close()
			continue
		}
		logs.Debug("rpop from redis succ, data:%s", string(data))
		if err != nil {
			logs.Error("rpop failed, err:%v", err)
			conn.Close()
			continue
		}

		var result SecResult
		err = json.Unmarshal([]byte(data), &result)
		if err != nil {
			logs.Error("json.Unmarshal failed, err:%v", err)
			conn.Close()
			continue
		}

		userKey := fmt.Sprintf("%v_%v", result.UserId, result.ProductID)

		secKillConf.UserConnMapLock.Lock()
		resultChan, ok := secKillConf.UserConnMap[userKey]
		secKillConf.UserConnMapLock.Unlock()
		if !ok {
			conn.Close()
			logs.Warn("user not found:%v", userKey)
			continue
		}

		resultChan <- &result //req.ResultChan会接收到
		conn.Close()
	}
}
