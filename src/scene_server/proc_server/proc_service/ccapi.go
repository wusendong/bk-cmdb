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

package ccapi

import (
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/core/cc/api"
	"configcenter/src/common/core/cc/config"
	"configcenter/src/common/errors"
	"configcenter/src/common/http/httpserver"
	"configcenter/src/common/language"
	"configcenter/src/common/metric"
	"configcenter/src/common/rdapi"
	"configcenter/src/common/types"
	confCenter "configcenter/src/scene_server/proc_server/proc_service/config"
	"configcenter/src/scene_server/proc_server/proc_service/rdiscover"
	"github.com/emicklei/go-restful"
	"time"
)

//CCAPIServer define data struct of bcs ccapi server
type CCAPIServer struct {
	conf     *config.CCAPIConfig
	httpServ *httpserver.HttpServer
	rd       *rdiscover.RegDiscover
	cfCenter *confCenter.ConfCenter
}

func NewCCAPIServer(conf *config.CCAPIConfig) (*CCAPIServer, error) {
	s := &CCAPIServer{}

	//config
	s.conf = conf
	addr, _ := s.conf.GetAddress()
	port, _ := s.conf.GetPort()

	//http server
	s.httpServ = httpserver.NewHttpServer(port, addr, "")

	a := api.NewAPIResource()
	a.SetConfig(s.conf)
	a.InitAction()

	s.rd = rdiscover.NewRegDiscover(s.conf.RegDiscover, addr, port, false)

	//ConfCenter
	s.cfCenter = confCenter.NewConfCenter(s.conf.RegDiscover)
	return s, nil
}

//Stop the ccapi server
func (ccAPI *CCAPIServer) Stop() error {
	return nil
}

//Start the ccapi server
func (ccAPI *CCAPIServer) Start() error {
	chErr := make(chan error, 3)

	a := api.NewAPIResource()

	// configure center
	go func() {
		err := ccAPI.cfCenter.Start()
		blog.Errorf("configure center module start failed!. err:%s", err.Error())
		chErr <- err
	}()

	/// fetch config of itselft
	var confData []byte
	var config map[string]string
	for {
		// temp code, just to debug
		if ccAPI.conf.ExConfig != "" {
			config, _ = a.ParseConfig()
			break
		}
		// end temp code

		confData = ccAPI.cfCenter.GetConfigureCxt()
		if confData == nil {
			blog.Warnf("fail to get configure, will get again")
			time.Sleep(time.Second * 2)
			continue
		} else {
			blog.Infof("get configure. ctx(%s)", string(confData))
			config, _ = a.ParseConf(confData)
			break
		}
	}

	//http server
	ccAPI.initHttpServ()

	go func() {
		err := ccAPI.httpServ.ListenAndServe()
		blog.Error("http listen and serve failed! err:%s", err.Error())
		chErr <- err
	}()

	// load the errors resource
	if errorres, ok := config["errors.res"]; ok {
		if errif, err := errors.New(errorres); nil != err {
			blog.Error("failed to create errors object, error info is  %s ", err.Error())
		} else {
			a.Error = errif
		}
	} else {
		for {
			errcode := ccAPI.cfCenter.GetErrorCxt()
			if errcode == nil {
				blog.Warnf("fail to get error package, will get again")
				time.Sleep(time.Second * 2)
				continue
			} else {
				errif := errors.NewFromCtx(errcode)
				a.Error = errif
				blog.Info("lanugage package loaded")
				break
			}
		}
	}

	// load the language resource
	if dirPath, ok := config["language.res"]; ok {
		if res, err := language.New(dirPath); nil != err {
			blog.Error("failed to create language object, error info is  %s ", err.Error())
			chErr <- err
		} else {
			a.Lang = res
		}
	} else {
		for {
			langCtx := ccAPI.cfCenter.GetLanguageResCxt()
			if langCtx == nil {
				blog.Warnf("fail to get language package, will get again")
				time.Sleep(time.Second * 2)
				continue
			} else {
				languageif := language.NewFromCtx(langCtx)
				a.Lang = languageif
				blog.Info("lanugage package loaded")
				break
			}
		}
	}

	a.AddrSrv = ccAPI.rd
	a.HostCtrl = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_HOSTCONTROLLER, a.AddrSrv)

	a.ObjCtrl = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_OBJECTCONTROLLER, a.AddrSrv)

	a.AuditCtrl = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_AUDITCONTROLLER, a.AddrSrv)

	a.ProcCtrl = rdapi.GetRdAddrSrvHandle(types.CC_MODULE_PROCCONTROLLER, a.AddrSrv)

	//RDiscover
	go func() {
		err := ccAPI.rd.Start()
		blog.Errorf("rdiscover start failed! err:%s", err.Error())
		chErr <- err
	}()

	select {
	case err := <-chErr:
		blog.Error("exit! err:%s", err.Error())
		return err
	}

	return nil
}

func (ccAPI *CCAPIServer) initHttpServ() error {
	a := api.NewAPIResource()
	getErrFun := func() errors.CCErrorIf {
		return a.Error
	}
	ccAPI.httpServ.RegisterWebServer("/process/{version}", rdapi.AllGlobalFilter(getErrFun()), a.Actions)
	// MetricServer
	conf := metric.Config{
		ModuleName:    types.CC_MODULE_PROC,
		ServerAddress: ccAPI.conf.AddrPort,
	}
	metricActions := metric.NewMetricController(conf, ccAPI.HealthMetric)
	as := []*httpserver.Action{}
	for _, metricAction := range metricActions {
		newmetricAction := metricAction
		as = append(as, &httpserver.Action{Verb: common.HTTPSelectGet, Path: newmetricAction.Path, Handler: func(req *restful.Request, resp *restful.Response) {
			newmetricAction.HandlerFunc(resp.ResponseWriter, req.Request)
		}})
	}
	ccAPI.httpServ.RegisterWebServer("/", nil, as)
	return nil
}

// HealthMetric check netservice is health
func (ccAPI *CCAPIServer) HealthMetric() metric.HealthMeta {
	a := api.GetAPIResource()
	meta := metric.HealthMeta{IsHealthy: true}

	// check zk
	meta.Items = append(meta.Items, metric.NewHealthItem(types.CCFunctionalityServicediscover, ccAPI.rd.Ping()))

	// check dependence
	meta.Items = append(meta.Items, metric.NewHealthItem(types.CC_MODULE_OBJECTCONTROLLER, metric.CheckHealthy(a.ObjCtrl())))
	meta.Items = append(meta.Items, metric.NewHealthItem(types.CC_MODULE_AUDITCONTROLLER, metric.CheckHealthy(a.AuditCtrl())))
	meta.Items = append(meta.Items, metric.NewHealthItem(types.CC_MODULE_HOSTCONTROLLER, metric.CheckHealthy(a.HostCtrl())))
	meta.Items = append(meta.Items, metric.NewHealthItem(types.CC_MODULE_PROCCONTROLLER, metric.CheckHealthy(a.ProcCtrl())))

	for _, item := range meta.Items {
		if item.IsHealthy == false {
			meta.IsHealthy = false
			meta.Message = "procserver is not healthy"
			break
		}
	}

	return meta
}
