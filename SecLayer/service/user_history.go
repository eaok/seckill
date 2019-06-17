package service

import (
	"sync"
)

type UserBuyHistory struct {
	history map[int]int
	lock    sync.RWMutex
}

//GetProductBuyCount 获取用户已经购买该产品的个数
func (p *UserBuyHistory) GetProductBuyCount(ProductID int) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	count, _ := p.history[ProductID]
	return count
}

//Add history[ProductID]加上count
func (p *UserBuyHistory) Add(ProductID, count int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	cur, ok := p.history[ProductID]
	if !ok {
		cur = count
	} else {
		cur += count
	}

	p.history[ProductID] = cur
}
