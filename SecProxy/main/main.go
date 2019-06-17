package main

import (
	_ "seckill/SecProxy/router"

	"github.com/astaxie/beego"
)

func main() {
	err := initConfig()
	if err != nil {
		panic(err)
	}

	err = initSec()
	if err != nil {
		panic(err)
	}

	beego.Run()
}
