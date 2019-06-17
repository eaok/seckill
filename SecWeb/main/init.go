package main

import (
	"fmt"
	"seckill/SecWeb/model"
	"time"

	"github.com/astaxie/beego/logs"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	etcd_client "go.etcd.io/etcd/clientv3"
)

var Db *sqlx.DB
var EtcdClient *etcd_client.Client

func initDb() (err error) {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", AppConf.mysqlConf.UserName, AppConf.mysqlConf.Passwd,
		AppConf.mysqlConf.Host, AppConf.mysqlConf.Port, AppConf.mysqlConf.Database)

	//Open 可能只是验证这些参数，并不会去连接database，要验证这个连接是否成功，使用ping()方法
	database, err := sqlx.Open("mysql", dns)
	if err != nil {
		logs.Error("open mysql failed, err:%v ", err)
		return
	}
	if err = database.Ping(); err != nil {
		logs.Error("connet mysql failed, err:%v ", err)
		return
	}

	Db = database
	logs.Debug("connect to mysql succ")
	return
}

func initEtcd() (err error) {
	cli, err := etcd_client.New(etcd_client.Config{
		Endpoints:   []string{AppConf.etcdConf.Addr},
		DialTimeout: time.Duration(AppConf.etcdConf.Timeout) * time.Second,
	})
	if err != nil {
		logs.Error("connect etcd failed, err:", err)
		return
	}

	EtcdClient = cli
	logs.Debug("init etcd succ")
	return
}

func initAll() (err error) {
	err = initConfig()
	if err != nil {
		logs.Warn("init config failed, err:%v", err)
		return
	}

	err = initDb()
	if err != nil {
		logs.Warn("init Db failed, err:%v", err)
		return
	}

	err = initEtcd()
	if err != nil {
		logs.Warn("init etcd failed, err:%v", err)
		return
	}

	err = model.Init(Db, EtcdClient, AppConf.etcdConf.EtcdKeyPrefix, AppConf.etcdConf.ProductKey)
	if err != nil {
		logs.Warn("init model failed, err:%v", err)
		return
	}
	return
}
