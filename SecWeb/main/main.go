package main

import (
	"fmt"
	_ "seckill/SecWeb/router"

	"github.com/astaxie/beego"
)

func main() {
	err := initAll()
	if err != nil {
		panic(fmt.Sprintf("init database failed, err:%v", err))
	}
	beego.Run()
}
