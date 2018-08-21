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

package app

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"configcenter/src/common/blog"

	restful "github.com/emicklei/go-restful"
	redis "gopkg.in/redis.v5"

	"configcenter/src/apimachinery"
	"configcenter/src/apimachinery/util"
	"configcenter/src/common"
	"configcenter/src/common/backbone"
	cc "configcenter/src/common/backbone/configcenter"
	"configcenter/src/common/types"
	"configcenter/src/common/version"
	"configcenter/src/source_controller/objectcontroller/app/options"
	"configcenter/src/source_controller/objectcontroller/service"
	"configcenter/src/storage/dal/mongo"
	dalredis "configcenter/src/storage/dal/redis"
)

//Run ccapi server
func Run(ctx context.Context, op *options.ServerOption) error {
	svrInfo, err := newServerInfo(op)
	if err != nil {
		return fmt.Errorf("wrap server info failed, err: %v", err)
	}

	c := &util.APIMachineryConfig{
		ZkAddr:    op.ServConf.RegDiscover,
		QPS:       1000,
		Burst:     2000,
		TLSConfig: nil,
	}

	machinery, err := apimachinery.NewApiMachinery(c)
	if err != nil {
		return fmt.Errorf("new api machinery failed, err: %v", err)
	}

	coreService := new(service.Service)
	server := backbone.Server{
		ListenAddr: svrInfo.IP,
		ListenPort: svrInfo.Port,
		Handler:    restful.NewContainer().Add(coreService.WebService()),
		TLS:        backbone.TLSConfig{},
	}

	regPath := fmt.Sprintf("%s/%s/%s", types.CC_SERV_BASEPATH, types.CC_MODULE_OBJECTCONTROLLER, svrInfo.IP)
	bonC := &backbone.Config{
		RegisterPath: regPath,
		RegisterInfo: *svrInfo,
		CoreAPI:      machinery,
		Server:       server,
	}

	objCtr := new(ObjectController)
	objCtr.Service = coreService
	objCtr.Service.Core, err = backbone.NewBackbone(ctx, op.ServConf.RegDiscover,
		types.CC_MODULE_HOSTCONTROLLER,
		op.ServConf.ExConfig,
		objCtr.onObjectConfigUpdate,
		bonC)
	if err != nil {
		return fmt.Errorf("new backbone failed, err: %v", err)
	}
	configReady := false
	for sleepCnt := 0; sleepCnt < common.APPConfigWaitTime; sleepCnt++ {
		if nil == objCtr.Config {
			time.Sleep(time.Second)
		} else {
			configReady = true
			break
		}
	}
	if false == configReady {
		return fmt.Errorf("Configuration item not found")
	}

	select {
	case <-ctx.Done():
	}
	return nil
}

type ObjectController struct {
	*service.Service
	Config *options.Config
}

func (h *ObjectController) onObjectConfigUpdate(previous, current cc.ProcessConfig) {

	mongocfg := mongo.Config{
		Address:      current.ConfigMap["mongo.address"],
		User:         current.ConfigMap["mongo.usr"],
		Password:     current.ConfigMap["mongo.pwd"],
		Database:     current.ConfigMap["mongo.database"],
		MaxOpenConns: current.ConfigMap["mongo.maxOpenConns"],
		MaxIdleConns: current.ConfigMap["mongo.maxIDleConns"],
		Mechanism:    current.ConfigMap["mongo.mechanism"],
	}

	rediscfg := dalredis.Config{
		Address:  current.ConfigMap["redis.address"],
		Password: current.ConfigMap["redis.pwd"],
		Database: current.ConfigMap["redis.database"],
	}

	h.Config = &options.Config{
		Mongo: mongocfg,
		Redis: rediscfg,
	}

	instance, err := mongo.NewMgo(h.Config.Mongo.BuildURI())
	if err != nil {
		blog.Errorf("new mongo client failed, err: %v", err)
		return
	}
	h.Service.Instance = instance

	rdsc := h.Config.Redis
	dbNum, err := strconv.Atoi(rdsc.Database)
	//not set use default db num 0
	if nil != err {
		blog.Errorf("redis config db[%s] not integer", rdsc.Database)
		return
	}
	h.Cache = redis.NewClient(
		&redis.Options{
			Addr:     rdsc.Address,
			PoolSize: 100,
			Password: rdsc.Password,
			DB:       dbNum,
		})
	err = h.Cache.Ping().Err()
	if err != nil {
		blog.Errorf("new redis client failed, err: %v", err)
		return
	}

}

func newServerInfo(op *options.ServerOption) (*types.ServerInfo, error) {
	ip, err := op.ServConf.GetAddress()
	if err != nil {
		return nil, err
	}

	port, err := op.ServConf.GetPort()
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	info := &types.ServerInfo{
		IP:       ip,
		Port:     port,
		HostName: hostname,
		Scheme:   "http",
		Version:  version.GetVersion(),
		Pid:      os.Getpid(),
	}
	return info, nil
}
