package service

import (
	"fmt"
	"sync"
)

type ProductCountMgr struct {
	productCount map[int]int
	lock         sync.RWMutex
}

//NewProductCountMgr 初始化一个ProductCountMgr指针
func NewProductCountMgr() (productMgr *ProductCountMgr) {
	productMgr = &ProductCountMgr{
		productCount: make(map[int]int, 128),
	}

	return
}

//Count 获取productCount[ProductID]
func (p *ProductCountMgr) Count(ProductID int) (count int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	count = p.productCount[ProductID]
	return

}

//Add productCount[ProductID]加上count
func (p *ProductCountMgr) Add(ProductID, count int) {

	p.lock.Lock()
	defer p.lock.Unlock()

	cur, ok := p.productCount[ProductID]
	if !ok {
		fmt.Printf("product_id:%v cur:%v\n, map:%v", ProductID, cur, p.productCount)
		cur = count
	} else {
		fmt.Printf("else product_id:%v cur:%v, map:%v\n", ProductID, cur, p.productCount)
		cur += count
	}

	p.productCount[ProductID] = cur
}
