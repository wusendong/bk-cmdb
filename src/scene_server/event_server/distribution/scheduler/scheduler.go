/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Scheduler 事件推送调度器
//
// 调度目标：
// - 各待处理队列尽量平均分布于多个进程
// - 进程或待处理队列数量增减后，尽量保持多数待处理队列在原处理进程上
//
// 如现有20各待处理队列， 2个处理进程，那么调度结果如下：
// ````
// pid 1 handling [001 004 006 008 010 012 014 016 018 020]
// pid 2 handling [002 003 005 007 009 011 013 015 017 019]
// ````
//
// 后来待处理队列增加到30个， 处理进程增加到4个，调度结果应变为这样：
// ````
// pid 1 handling [001 004 006 008 010 012 014]
// pid 2 handling [002 003 005 007 009 011 013]
// pid 3 handling [016 017 018 019 020 21 22 23]
// pid 4 handling [015 24 25 26 27 28 29 30]
// ````
type Scheduler struct {
}

func New() *Scheduler {
	s := Scheduler{}
	return &s
}

func (s *Scheduler) Run() {

}

func main() {
	subID := 1
	for ; subID <= 20; subID++ {
		allsubsMutes.Lock()
		allsubs = append(allsubs, fmt.Sprintf("%.3d", subID))
		allsubsMutes.Unlock()
	}

	pid := 1
	for ; pid < 3; pid++ {
		p := &process{id: pid, handMap: map[string]chan struct{}{}}
		dealmap[p.id] = nil
		go p.start()
	}

	go func() {
		for {
			time.Sleep(time.Second * 5)
			p := &process{id: pid, handMap: map[string]chan struct{}{}}
			dealmap[p.id] = nil
			go p.start()
			pid++
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second * 1)
			allsubsMutes.Lock()
			allsubs = append(allsubs, strconv.Itoa(subID))
			allsubsMutes.Unlock()
			subID++
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Print("done")
}

type process struct {
	id      int
	handMap map[string]chan struct{}
}

func (p *process) start() {
	if declareMaster(p.id) {
		go schedule()
	}

	for _, sub := range dealmap[p.id] {
		stopChan := make(chan struct{})
		p.handMap[sub] = stopChan
		go p.dist(sub, stopChan)
	}

	tick := time.NewTicker(time.Second)
	for {
		select {
		case <-tick.C:
			p.reconcil()
		}
	}
}

type deal struct {
	id   int
	subs []string
}

func schedule() {
	for range time.Tick(time.Second) {

		distmap := map[string]int{}
		var ds []deal
		for pid, dists := range dealmap {
			sort.Strings(dists)
			ds = append(ds, deal{pid, dists})
			for _, dist := range dists {
				distmap[dist] = pid
			}
		}

		var unrunsubs []string
		allsubsMutes.Lock()
		for _, sub := range allsubs {
			if _, ok := distmap[sub]; !ok {
				unrunsubs = append(unrunsubs, sub)
			}
		}
		allsubsMutes.Unlock()

		for _, sub := range unrunsubs {
			sort.Slice(ds, func(i, j int) bool {
				if len(ds[i].subs) == len(ds[j].subs) {
					for si := range ds[i].subs {
						if ds[i].subs[si] == ds[j].subs[si] {
							continue
						}
						return ds[i].subs[si] > ds[j].subs[si]
					}
				}
				return len(ds[i].subs) < len(ds[j].subs)
			})

			if len(ds) > 0 {
				dealmap[ds[0].id] = append(dealmap[ds[0].id], sub)
				ds[0].subs = append(ds[0].subs, sub)
			}
		}

		pcount := len(ds)
		allsubsMutes.Lock()
		avg := len(allsubs) / pcount
		allsubsMutes.Unlock()

		for i := range ds {
			id := ds[i].id
			for len(ds[i].subs) > avg+1 {
				sub := ds[i].subs[len(ds[i].subs)-1]
				ds[i].subs = ds[i].subs[:len(ds[i].subs)-1]
				if len(dealmap[id]) > 0 {
					dealmap[id] = dealmap[id][:len(dealmap[id])-1]
				}
				if len(ds) > 0 {
					ds[0].subs = append(ds[0].subs, sub)
					dealmap[ds[0].id] = append(dealmap[ds[0].id], sub)
					sort.Strings(dealmap[ds[0].id])
				}
				sortds(ds)
			}
		}

		for ii := range ds {
			for len(ds[ii].subs) < avg {
				for i := range ds {
					id := ds[i].id
					if len(ds[i].subs) > avg {
						sub := ds[i].subs[len(ds[i].subs)-1]
						ds[i].subs = ds[i].subs[:len(ds[i].subs)-1]
						if len(dealmap[id]) > 0 {
							dealmap[id] = dealmap[id][:len(dealmap[id])-1]
						}
						if len(ds) > 0 {
							ds[0].subs = append(ds[0].subs, sub)
							dealmap[ds[0].id] = append(dealmap[ds[0].id], sub)
							sort.Strings(dealmap[ds[0].id])
						}
						sortds(ds)
					}

				}
			}
		}

		for i := 1; i <= len(dealmap); i++ {
			fmt.Printf("pid %d handling %v\n", i, dealmap[i])
		}
		fmt.Println("")
	}
}

func sortds(ds []deal) {
	sort.Slice(ds, func(i, j int) bool {
		if len(ds[i].subs) == len(ds[j].subs) {
			for si := range ds[i].subs {
				if ds[i].subs[si] == ds[j].subs[si] {
					continue
				}
				return ds[i].subs[si] > ds[j].subs[si]
			}
		}
		return len(ds[i].subs) < len(ds[j].subs)
	})
}

func (p *process) reconcil() {
	distMap := map[string]bool{}
	for _, sub := range dealmap[p.id] {
		distMap[sub] = true
		if _, ok := p.handMap[sub]; !ok {
			stopChan := make(chan struct{})
			p.handMap[sub] = stopChan
			go p.dist(sub, stopChan)
		}
	}
	for sub, stopChan := range p.handMap {
		if _, ok := distMap[sub]; !ok {
			close(stopChan)
		}
	}
}

func (p *process) dist(sid string, stopChan chan struct{}) {
	// log.Printf("process %d handling %s", p.id, sid)
	select {
	case <-stopChan:
		delete(p.handMap, sid)
	}
	// log.Printf("process %d stop handling %s", p.id, sid)
}

var allsubsMutes sync.Mutex
var allsubs = []string{}

var dealmap = map[int][]string{}

var stopmap = map[string]interface{}{}

var nxlock sync.Mutex

func setNX(m map[int]string, key int, value string) bool {
	nxlock.Lock()
	defer nxlock.Unlock()
	_, ok := m[key]
	if !ok {
		m[key] = value
	}
	return !ok
}

var mslock sync.Mutex
var ms int

func declareMaster(id int) bool {
	mslock.Lock()
	defer mslock.Unlock()
	if ms == 0 {
		ms = id
	}
	return ms == id
}
