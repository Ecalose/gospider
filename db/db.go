package db

import (
	"context"
	"errors"
	"sync"
	"time"

	"gitee.com/baixudong/gospider/chanx"
)

type Client[T any] struct {
	orderKey  *chanx.Client[dbKey]
	mapKey    map[[16]byte]dbData[T]
	lock      sync.RWMutex
	timeOut   time.Duration
	ctx       context.Context
	cnl       context.CancelFunc
	afterTime *time.Timer
}
type dbKey struct {
	key [16]byte
	ttl time.Time
}
type dbData[T any] struct {
	data T
	ttl  time.Time
}

// 缓存数据
func NewClient[T any](preCtx context.Context, timeOuts ...time.Duration) *Client[T] {
	var timeOut time.Duration
	if len(timeOuts) > 0 {
		timeOut = timeOuts[0]
	} else {
		timeOut = time.Second * 60 * 30
	}
	ctx, cnl := context.WithCancel(preCtx)
	client := &Client[T]{
		ctx:       ctx,
		cnl:       cnl,
		timeOut:   timeOut,
		mapKey:    make(map[[16]byte]dbData[T]),
		orderKey:  chanx.NewClient[dbKey](ctx),
		afterTime: time.NewTimer(0),
	}
	go client.run()
	return client
}

func (obj *Client[T]) run() {
	defer obj.Close()
	for {
		select {
		case <-obj.ctx.Done():
			return
		case <-obj.orderKey.Ctx().Done():
			return
		case orderVal := <-obj.orderKey.Chan():
			if awaitTime := obj.timeOut - (time.Now().Sub(orderVal.ttl)); awaitTime > 0 { //判断睡眠时间
				obj.afterTime.Reset(awaitTime)
				select {
				case <-obj.ctx.Done():
					return
				case <-obj.orderKey.Ctx().Done():
					return
				case <-obj.afterTime.C:
				}
			}
			obj.lock.Lock()
			delete(obj.mapKey, orderVal.key)
			obj.lock.Unlock()
		}
	}
}
func (obj *Client[T]) Close() {
	obj.cnl()
	obj.afterTime.Stop()
}
func (obj *Client[T]) Put(key [16]byte, value T) error {
	nowTime := time.Now()
	if err := obj.orderKey.Add(dbKey{key: key, ttl: nowTime}); err != nil {
		return err
	}
	obj.lock.Lock()
	defer obj.lock.Unlock()
	obj.mapKey[key] = dbData[T]{data: value, ttl: nowTime}
	return nil
}
func (obj *Client[T]) Get(key [16]byte) (value T, err error) {
	obj.lock.RLock()
	mapVal, ok := obj.mapKey[key]
	obj.lock.RUnlock()
	if ok {
		value = mapVal.data
	} else {
		err = errors.New("not found")
	}
	return
}
