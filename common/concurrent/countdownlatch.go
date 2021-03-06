package concurrent

import (
	"fmt"
	"sync"
	"time"
)

type CountDownLatch interface {
	Cancel()
	WaitTimeout(duration time.Duration) bool
	Wait()
	Done()
	IsAlive() bool
}

type countDownLatch struct {
	lock     sync.Mutex
	cond     *sync.Cond
	ch       chan struct{}
	num      int
	canceled bool
}

func (l *countDownLatch) Cancel() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.canceled {
		l.canceled = true
		l.ch <- struct{}{}
	} else {
		fmt.Errorf("error: already canceled")
	}
}

func (l *countDownLatch) WaitTimeout(duration time.Duration) bool {
	select {
	case <-l.ch:
		return true
	case <-time.After(duration):
		return false
	}
	// TODO was going to modify these in Sept
}

func (l *countDownLatch) Wait() {
	select {
	case <-l.ch:
	}
}

func (l *countDownLatch) Done() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.canceled {
		fmt.Errorf("error: already canceled. Cannot done")
		return
	}
	if l.num == 0 {
		fmt.Errorf("error: already done")
		return
	}
	l.num--
	if l.num == 0 {
		l.ch <- struct{}{}
	}
}

func (l *countDownLatch) IsAlive() bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	return !l.canceled && l.num > 0
}

func NewCountDownLatch(num int) CountDownLatch {
	c := &countDownLatch{num: num, canceled: false}
	c.cond = sync.NewCond(&c.lock)
	c.ch = make(chan struct{}, 1) // 1 is necessary
	return c
}
