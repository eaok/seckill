package controller

import (
	"seckill/SecProxy/service"
	"strings"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

//SkillController Skill控制器
type SkillController struct {
	beego.Controller
}

//SecKill 路由/seckill
func (p *SkillController) SecKill() {
	result := make(map[string]interface{})
	result["code"] = 0
	result["message"] = "success"

	defer func() {
		p.Data["json"] = result
		p.ServeJSON()
	}()

	ProductID, err := p.GetInt("product_id")
	if err != nil {
		result["code"] = 1001
		result["message"] = "invalid product_id"
		return
	}

	source := p.GetString("src")
	authcode := p.GetString("authcode")
	secTime := p.GetString("time")
	nance := p.GetString("nance")

	//组装请求信息系
	secRequest := service.NewSecRequest()
	secRequest.AuthCode = authcode
	secRequest.Nance = nance
	secRequest.ProductID = ProductID
	secRequest.SecTime = secTime
	secRequest.Source = source
	secRequest.UserAuthSign = p.Ctx.GetCookie("userAuthSign")
	secRequest.UserId, _ = p.GetInt("user_id")
	secRequest.AccessTime = time.Now()
	if len(p.Ctx.Request.RemoteAddr) > 0 {
		secRequest.ClientAddr = strings.Split(p.Ctx.Request.RemoteAddr, ":")[0]
	}
	secRequest.ClientRefence = p.Ctx.Request.Referer()
	secRequest.CloseNotify = p.Ctx.ResponseWriter.CloseNotify()
	logs.Debug("client request:[%v]", secRequest)

	//调用service里面的信息
	data, code, err := service.SecKill(secRequest)
	if err != nil {
		result["code"] = code
		result["message"] = err.Error()
		return
	}

	result["data"] = data
	result["code"] = code

	return
}

//SecInfo 路由/secinfo
func (p *SkillController) SecInfo() {
	productID, err := p.GetInt("product_id")
	result := make(map[string]interface{})

	result["code"] = 0
	result["message"] = "success"

	defer func() {
		p.Data["json"] = result
		p.ServeJSON()
	}()

	if err != nil {
		data, code, err := service.SecInfoList()
		if err != nil {
			result["code"] = code
			result["message"] = err.Error()

			logs.Error("invalid request, get product_id failed, err:%v", err)
			return
		}

		result["code"] = code
		result["data"] = data
	} else {
		data, code, err := service.SecInfo(productID)
		if err != nil {
			result["code"] = code
			result["message"] = err.Error()

			logs.Error("invalid request, get product_id failed, err:%v", err)
			return
		}

		result["code"] = code
		result["data"] = data
	}

}
