package service

import (
	"strconv"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/garyburd/redigo/redis"
)

var (
	secKillConf *SecSkillConf
)

//InitService 初始化服务
func InitService(serviceConf *SecSkillConf) (err error) {
	secKillConf = serviceConf

	err = loadBlackList()
	if err != nil {
		logs.Error("load black list err:%v", err)
		return
	}
	logs.Debug("init service succ, config:%v", secKillConf)

	err = initProxy2LayerRedis()
	if err != nil {
		logs.Error("load proxy2layer redis pool failed, err:%v", err)
		return
	}

	secKillConf.secLimitMgr = &SecLimitMgr{
		UserLimitMap: make(map[int]*Limit, 10000),
		IPLimitMap:   make(map[string]*Limit, 10000),
	}
	secKillConf.SecReqChan = make(chan *SecRequest, secKillConf.SecReqChanSize)
	secKillConf.UserConnMap = make(map[string]chan *SecResult, 10000)

	initRedisProcessFunc()

	return
}

//initRedisProcessFunc 初试化redis读写线程
func initRedisProcessFunc() {
	for i := 0; i < secKillConf.WriteProxy2LayerGoroutineNum; i++ {
		go WriteHandle()
	}

	for i := 0; i < secKillConf.ReadProxy2LayerGoroutineNum; i++ {
		go ReadHandle()
	}
}

//initProxy2LayerRedis 初始化接入层到逻辑层的redis
func initProxy2LayerRedis() (err error) {
	secKillConf.proxy2LayerRedisPool = &redis.Pool{
		MaxIdle:     secKillConf.RedisProxy2LayerConf.RedisMaxIdle,
		MaxActive:   secKillConf.RedisProxy2LayerConf.RedisMaxActive,
		IdleTimeout: time.Duration(secKillConf.RedisProxy2LayerConf.RedisIdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", secKillConf.RedisProxy2LayerConf.RedisAddr, redis.DialPassword("123456"))
		},
	}

	conn := secKillConf.proxy2LayerRedisPool.Get()
	defer conn.Close()

	_, err = conn.Do("ping")
	if err != nil {
		logs.Error("ping redis failed, err:%v", err)
		return
	}

	return
}

//initLayer2ProxyRedis 初始化逻辑层到接入层的redis
func initLayer2ProxyRedis() (err error) {
	secKillConf.layer2ProxyRedisPool = &redis.Pool{
		MaxIdle:     secKillConf.RedisLayer2ProxyConf.RedisMaxIdle,
		MaxActive:   secKillConf.RedisLayer2ProxyConf.RedisMaxActive,
		IdleTimeout: time.Duration(secKillConf.RedisLayer2ProxyConf.RedisIdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", secKillConf.RedisLayer2ProxyConf.RedisAddr, redis.DialPassword("123456"))
		},
	}

	conn := secKillConf.layer2ProxyRedisPool.Get()
	defer conn.Close()

	_, err = conn.Do("ping")
	if err != nil {
		logs.Error("ping redis failed, err:%v", err)
		return
	}

	return
}

//initBlackRedis 初始化黑名单(ip/id)
func initBlackRedis() (err error) {
	secKillConf.blackRedisPool = &redis.Pool{
		MaxIdle:     secKillConf.RedisBlackConf.RedisMaxIdle,
		MaxActive:   secKillConf.RedisBlackConf.RedisMaxActive,
		IdleTimeout: time.Duration(secKillConf.RedisBlackConf.RedisIdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", secKillConf.RedisBlackConf.RedisAddr, redis.DialPassword("123456"))
		},
	}

	conn := secKillConf.blackRedisPool.Get()
	defer conn.Close()

	_, err = conn.Do("ping")
	if err != nil {
		logs.Error("ping redis failed, err:%v", err)
		return
	}

	return
}

//loadBlackList 加载黑名单
func loadBlackList() (err error) {
	secKillConf.ipBlackMap = make(map[string]bool, 10000)
	secKillConf.idBlackMap = make(map[int]bool, 10000)

	err = initBlackRedis()
	if err != nil {
		logs.Error("init black redis failed, err:%v", err)
		return
	}

	conn := secKillConf.blackRedisPool.Get()
	defer conn.Close()

	reply, err := conn.Do("hgetall", "idblacklist")
	idlist, err := redis.Strings(reply, err)
	if err != nil {
		logs.Warn("hget all failed, err:%v", err)
		return
	}

	for _, v := range idlist {
		id, err := strconv.Atoi(v)
		if err != nil {
			logs.Warn("invalid user id [%v]", id)
			continue
		}
		secKillConf.idBlackMap[id] = true
	}

	reply, err = conn.Do("hgetall", "ipblacklist")
	iplist, err := redis.Strings(reply, err)
	if err != nil {
		logs.Warn("hget all failed, err:%v", err)
		return
	}

	for _, v := range iplist {
		secKillConf.ipBlackMap[v] = true
	}

	go SyncIPBlackList()
	go SyncIDBlackList()
	return
}

//SyncIPBlackList 同步黑名单ip
func SyncIPBlackList() {
	var ipList []string
	lastTime := time.Now().Unix()
	for {
		conn := secKillConf.blackRedisPool.Get()
		defer conn.Close()
		reply, err := conn.Do("BLPOP", "blackiplist", time.Second)
		ip, err := redis.String(reply, err)
		if err != nil {
			continue
		}

		curTime := time.Now().Unix()
		ipList = append(ipList, ip)

		if len(ipList) > 100 || curTime-lastTime > 5 {
			secKillConf.RWBlackLock.Lock()
			for _, v := range ipList {
				secKillConf.ipBlackMap[v] = true
			}
			secKillConf.RWBlackLock.Unlock()

			lastTime = curTime
			logs.Info("sync ip list from redis succ, ip[%v]", ipList)
		}
	}
}

//SyncIDBlackList 同步黑名单id
func SyncIDBlackList() {
	for {
		conn := secKillConf.blackRedisPool.Get()
		defer conn.Close()
		reply, err := conn.Do("BLPOP", "blackidlist", time.Second)
		id, err := redis.Int(reply, err)
		if err != nil {
			continue
		}

		secKillConf.RWBlackLock.Lock()
		secKillConf.idBlackMap[id] = true
		secKillConf.RWBlackLock.Unlock()

		logs.Info("sync id list from redis succ, ip[%v]", id)
	}
}
