# seckill

* 系统架构图
![系统架构图](http://kcoewoys.maser.top/github/seckill/%E7%A7%92%E6%9D%80%E7%B3%BB%E7%BB%9F%E6%9E%B6%E6%9E%84%E5%9B%BE.png)
* 秒杀图
![秒杀图](http://kcoewoys.maser.top/github/seckill/%E7%A7%92%E6%9D%80%E5%9B%BE.png)

* 秒杀脑图
![秒杀脑图](http://kcoewoys.maser.top/github/seckill/seckill.png)


---

### 接入层和逻辑层
* 开启redis
* 开启etcd

编译并运行服务
```
> cd seckill\SecProxy
> go build seckill\SecProxy\main
> main
> cd ..\SecLayer
> go build seckill\SecLayer\main
> main
```

浏览器中访问
> http://localhost:9091/secinfo
>
> http://localhost:9091/seckill?product_id=1079


### Web管理层
* 开启etcd
* 开启mysql

编译并运行服务
```
> cd seckill\SecWeb
> go build seckill\SecWeb\main
> main
```

浏览器中访问
> http://localhost:9090