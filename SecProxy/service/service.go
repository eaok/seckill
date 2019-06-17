package service

import (
	"fmt"
	"time"

	"crypto/md5"

	"github.com/astaxie/beego/logs"
)

//NewSecRequest 初始化一个结构体指针SecRequest
func NewSecRequest() (secRequest *SecRequest) {
	secRequest = &SecRequest{
		ResultChan: make(chan *SecResult, 1),
	}

	return
}

//SecInfoList 获取多个产品id的信息
func SecInfoList() (data []map[string]interface{}, code int, err error) {
	secKillConf.RWSecProductLock.RLock()
	defer secKillConf.RWSecProductLock.RUnlock()

	for _, v := range secKillConf.SecProductInfoMap {
		item, _, err := SecInfoByID(v.ProductID)
		if err != nil {
			logs.Error("get product_id[%d] failed, err:%v", v.ProductID, err)
			continue
		}

		logs.Debug("get product[%d]， result[%v], all[%v] v[%v]", v.ProductID, item, secKillConf.SecProductInfoMap, v)
		data = append(data, item)
	}

	return
}

//SecInfo 获取产品id的信息并添加到data
func SecInfo(ProductID int) (data []map[string]interface{}, code int, err error) {
	secKillConf.RWSecProductLock.RLock()
	defer secKillConf.RWSecProductLock.RUnlock()

	item, code, err := SecInfoByID(ProductID)
	if err != nil {
		return
	}

	data = append(data, item)
	return
}

//SecInfoByID 获取产品id的信息
func SecInfoByID(ProductID int) (data map[string]interface{}, code int, err error) {
	secKillConf.RWSecProductLock.RLock()
	defer secKillConf.RWSecProductLock.RUnlock()

	v, ok := secKillConf.SecProductInfoMap[ProductID]
	if !ok {
		code = ErrNotFoundProductID
		err = fmt.Errorf("not found product_id:%d", ProductID)
		return
	}

	start := false
	end := false
	status := "success"

	now := time.Now().Unix()
	if now-v.StartTime < 0 {
		start = false
		end = false
		status = "sec kill is not start"
		code = ErrActiveNotStart
	}

	if now-v.StartTime > 0 {
		start = true
	}

	if now-v.EndTime > 0 {
		start = false
		end = true
		status = "sec kill is already end"
		code = ErrActiveAlreadyEnd
	}

	if v.Status == ProductStatusForceSaleOut || v.Status == ProductStatusSaleOut {
		start = false
		end = true
		status = "product is sale out"
		code = ErrActiveSaleOut
	}

	data = make(map[string]interface{})
	data["product_id"] = ProductID
	data["start"] = start
	data["end"] = end
	data["status"] = status

	return
}

//userCheck 校验用户
func userCheck(req *SecRequest) (err error) {
	//来源地址白名单
	found := false
	for _, refer := range secKillConf.ReferWhiteList {
		if refer == req.ClientRefence {
			found = true
			break
		}
	}
	if !found {
		err = fmt.Errorf("invalid request")
		logs.Warn("user[%d] is reject by refer, req[%v]", req.UserId, req)
		return
	}

	//验证用户是否有效
	authData := fmt.Sprintf("%d:%s", req.UserId, secKillConf.CookieSecretKey)
	authSign := fmt.Sprintf("%x", md5.Sum([]byte(authData)))
	if authSign != req.UserAuthSign {
		err = fmt.Errorf("invalid user cookie auth")
		return
	}
	return
}

//SecKill SecKill服务
func SecKill(req *SecRequest) (data map[string]interface{}, code int, err error) {
	secKillConf.RWSecProductLock.RLock()
	defer secKillConf.RWSecProductLock.RUnlock()

	// err = userCheck(req)
	// if err != nil {
	// 	code = ErrUserCheckAuthFailed
	// 	logs.Warn("userId[%d] invalid, check failed, req[%v]", req.UserId, req)
	// 	return
	// }

	err = antiSpam(req)
	if err != nil {
		code = ErrUserServiceBusy
		logs.Warn("userId[%d] invalid, check failed, req[%v]", req.UserId, req)
		return
	}

	data, code, err = SecInfoByID(req.ProductID)
	if err != nil {
		logs.Warn("userId[%d] secInfoBy Id failed, req[%v]", req.UserId, req)
		return
	}

	if code != 0 {
		logs.Warn("userId[%d] secInfoByid failed, code[%d] req[%v]", req.UserId, code, req)
		return
	}

	//写入redis队列 接入层--->业务逻辑层
	userKey := fmt.Sprintf("%v_%v", req.UserId, req.ProductID)
	secKillConf.UserConnMap[userKey] = req.ResultChan
	secKillConf.SecReqChan <- req

	ticker := time.NewTicker(time.Second * 10)

	defer func() {
		ticker.Stop()
		secKillConf.UserConnMapLock.Lock()
		delete(secKillConf.UserConnMap, userKey)
		secKillConf.UserConnMapLock.Unlock()
	}()

	select {
	case <-ticker.C:
		code = ErrProcessTimeout
		err = fmt.Errorf("request timeout")
		return

	case <-req.CloseNotify:
		code = ErrClientClosed
		err = fmt.Errorf("client already closed")
		return

	case result := <-req.ResultChan:
		code = result.Code
		data["product_id"] = result.ProductID
		data["token"] = result.Token
		data["user_id"] = result.UserId
		return
	}
}
