package main

import (
	"encoding/json"
	"fmt"
	"seckill/SecProxy/service"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/garyburd/redigo/redis"
	etcd_client "go.etcd.io/etcd/clientv3"
	"golang.org/x/net/context"
)

var (
	redisPool  *redis.Pool
	etcdClient *etcd_client.Client
)

//initEtcd 初始化etcd
func initEtcd() (err error) {
	cli, err := etcd_client.New(etcd_client.Config{
		Endpoints:   []string{secKillConf.EtcdConf.EtcdAddr},
		DialTimeout: time.Duration(secKillConf.EtcdConf.Timeout) * time.Second,
	})
	if err != nil {
		logs.Error("connect etcd failed, err:", err)
		return
	}

	etcdClient = cli
	return
}

func convertLogLevel(level string) int {
	switch level {
	case "debug":
		return logs.LevelDebug
	case "warn":
		return logs.LevelWarn
	case "info":
		return logs.LevelInfo
	case "trace":
		return logs.LevelTrace
	}

	return logs.LevelDebug
}

//initLogger 初试化日志
func initLogger() (err error) {
	config := make(map[string]interface{})
	config["filename"] = secKillConf.LogPath
	config["level"] = convertLogLevel(secKillConf.LogLevel)

	configStr, err := json.Marshal(config)
	if err != nil {
		fmt.Println("marshal failed, err:", err)
		return
	}

	logs.SetLogger(logs.AdapterFile, string(configStr))
	return
}

//loadSecConf 从etcd中读入产品信息
func loadSecConf() (err error) {
	resp, err := etcdClient.Get(context.Background(), secKillConf.EtcdConf.EtcdSecProductKey)
	if err != nil {
		logs.Error("get [%s] from etcd failed, err:%v", secKillConf.EtcdConf.EtcdSecProductKey, err)
		return
	}

	var secProductInfo []service.SecProductInfoConf
	for k, v := range resp.Kvs {
		logs.Debug("key[%v] valud[%v]", k, v)
		err = json.Unmarshal(v.Value, &secProductInfo)
		if err != nil {
			logs.Error("Unmarshal sec product info failed, err:%v", err)
			return
		}

		logs.Debug("sec info conf is [%v]", secProductInfo)
	}

	updateSecProductInfo(secProductInfo)
	return
}

//initSec 初始化
func initSec() (err error) {
	err = initLogger()
	if err != nil {
		logs.Error("init logger failed, err:%v", err)
		return
	}

	err = initEtcd()
	if err != nil {
		logs.Error("init etcd failed, err:%v", err)
		return
	}

	err = loadSecConf()
	if err != nil {
		logs.Error("load sec conf failed, err:%v", err)
		return
	}

	service.InitService(secKillConf)
	initSecProductWatcher()

	logs.Info("init sec succ")
	return
}

func initSecProductWatcher() {
	go watchSecProductKey(secKillConf.EtcdConf.EtcdSecProductKey)
}

// 监听etcd的变化
func watchSecProductKey(key string) {
	cli, err := etcd_client.New(etcd_client.Config{
		Endpoints:   []string{"localhost:2379", "localhost:22379", "localhost:32379"},
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		logs.Error("connect etcd failed, err:", err)
		return
	}

	logs.Debug("begin watch key:%s", key)
	for {
		rch := cli.Watch(context.Background(), key)
		var secProductInfo []service.SecProductInfoConf
		var getConfSucc = true

		for wresp := range rch {
			for _, ev := range wresp.Events {
				if ev.Type == mvccpb.DELETE {
					logs.Warn("key[%s] 's config deleted", key)
					continue
				}

				if ev.Type == mvccpb.PUT && string(ev.Kv.Key) == key {
					err = json.Unmarshal(ev.Kv.Value, &secProductInfo)
					if err != nil {
						logs.Error("key [%s], Unmarshal[%s], err:%v ", err)
						getConfSucc = false
						continue
					}
				}
				logs.Debug("get config from etcd, %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}

			if getConfSucc {
				logs.Debug("get config from etcd succ, %v", secProductInfo)
				updateSecProductInfo(secProductInfo)
			}
		}

	}
}

//updateSecProductInfo 将产品信息更新到secKillConf
func updateSecProductInfo(secProductInfo []service.SecProductInfoConf) {
	tmp := make(map[int]*service.SecProductInfoConf, 1024)
	for _, v := range secProductInfo {
		produtInfo := v
		tmp[v.ProductID] = &produtInfo
	}

	secKillConf.RWSecProductLock.Lock()
	secKillConf.SecProductInfoMap = tmp
	secKillConf.RWSecProductLock.Unlock()
}
