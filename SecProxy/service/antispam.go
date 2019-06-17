package service

import (
	"fmt"
	"sync"

	"github.com/astaxie/beego/logs"
)

type SecLimitMgr struct {
	UserLimitMap map[int]*Limit
	IPLimitMap   map[string]*Limit
	lock         sync.Mutex
}

//antiSpam 防止刷屏
func antiSpam(req *SecRequest) (err error) {
	//检验uid/ip是否在黑名单中
	_, ok := secKillConf.idBlackMap[req.UserId]
	if ok {
		err = fmt.Errorf("invalid request")
		logs.Error("useId[%v] is block by id black", req.UserId)
		return
	}
	_, ok = secKillConf.ipBlackMap[req.ClientAddr]
	if ok {
		err = fmt.Errorf("invalid request")
		logs.Error("useId[%v] ip[%v] is block by ip black", req.UserId, req.ClientAddr)
		return
	}

	secKillConf.secLimitMgr.lock.Lock()
	//uid 频率控制
	limit, ok := secKillConf.secLimitMgr.UserLimitMap[req.UserId]
	if !ok {
		limit = &Limit{
			secLimit: &SecLimit{},
			minLimit: &MinLimit{},
		}
		secKillConf.secLimitMgr.UserLimitMap[req.UserId] = limit
	}

	secIDCount := limit.secLimit.Count(req.AccessTime.Unix())
	minIDCount := limit.minLimit.Count(req.AccessTime.Unix())

	//ip 频率控制
	limit, ok = secKillConf.secLimitMgr.IPLimitMap[req.ClientAddr]
	if !ok {
		limit = &Limit{
			secLimit: &SecLimit{},
			minLimit: &MinLimit{},
		}
		secKillConf.secLimitMgr.IPLimitMap[req.ClientAddr] = limit
	}

	secIPCount := limit.secLimit.Count(req.AccessTime.Unix())
	minIPCount := limit.minLimit.Count(req.AccessTime.Unix())
	secKillConf.secLimitMgr.lock.Unlock()

	if secIPCount > secKillConf.AccessLimitConf.IPSecAccessLimit {
		err = fmt.Errorf("invalid request")
		return
	}

	if minIPCount > secKillConf.AccessLimitConf.IPMinAccessLimit {
		err = fmt.Errorf("invalid request")
		return
	}

	if secIDCount > secKillConf.AccessLimitConf.UserSecAccessLimit {
		err = fmt.Errorf("invalid request")
		return
	}

	if minIDCount > secKillConf.AccessLimitConf.UserMinAccessLimit {
		err = fmt.Errorf("invalid request")
		return
	}
	return
}
