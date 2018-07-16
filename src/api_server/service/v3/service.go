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

package v3

import (
    "io"
    "net/http"
    "strings"
    "errors"
    
    "configcenter/src/apimachinery/discovery"
    "configcenter/src/common"
    "configcenter/src/common/backbone"
    "configcenter/src/common/blog"
    "configcenter/src/common/metadata"
    cErr "configcenter/src/common/errors"
    "github.com/emicklei/go-restful"
    "configcenter/src/common/rdapi"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Service struct {
	Engine *backbone.Engine
	Client HttpClient
	Disc   discovery.DiscoveryInterface
}

const (
	rootPath = "/api/v3"
)

func (s *Service) V3WebService() *restful.WebService {
	ws := new(restful.WebService)
    getErrFun := func() cErr.CCErrorIf {
        return s.Engine.CCErr
    }
	ws.Path(rootPath).
		Filter(rdapi.AllGlobalFilter(getErrFun)).
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON)

	ws.Route(ws.GET("{.*}").Filter(s.URLFilterChan).To(s.Get))
	ws.Route(ws.POST("{.*}").Filter(s.URLFilterChan).To(s.Post))
	ws.Route(ws.PUT("{.*}").Filter(s.URLFilterChan).To(s.Put))
	ws.Route(ws.DELETE("{.*}").Filter(s.URLFilterChan).To(s.Delete))

	return ws
}

func (s *Service) Get(req *restful.Request, resp *restful.Response) {
	s.Do(req, resp)
}

func (s *Service) Put(req *restful.Request, resp *restful.Response) {
	s.Do(req, resp)
}

func (s *Service) Post(req *restful.Request, resp *restful.Response) {
	s.Do(req, resp)
}

func (s *Service) Delete(req *restful.Request, resp *restful.Response) {
	s.Do(req, resp)
}

func (s *Service) Do(req *restful.Request, resp *restful.Response) {
	response, err := s.Client.Do(req.Request)
	if err != nil {
		blog.Errorf("do request[url: %s] failed, err: %v", req.Request.RequestURI, err)

		if err := resp.WriteError(http.StatusBadGateway, &metadata.RespError{
			Msg:     errors.New("proxy request failed"),
			ErrCode: common.CCErrProxyRequestFailed,
			Data:    nil,
		}); err != nil {
			blog.Errorf("response request[url: %s] failed, err: %v", req.Request.RequestURI, err)
		}
		return
	}

	defer response.Body.Close()

	for k, v := range response.Header {
		if len(v) > 0 {
			resp.Header().Set(k, v[0])
		}
	}

	resp.ResponseWriter.WriteHeader(response.StatusCode)

	if _, err := io.Copy(resp, response.Body); err != nil {
		blog.Errorf("response request[url: %s] failed, err: %v", req.Request.RequestURI, err)
		return
	}

	return
}

func (s *Service) URLFilterChan(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	var kind RequestType
	var err error
	kind, err = V3URLPath(req.Request.RequestURI).FilterChain(req)
	if err != nil {
		blog.Errorf("rewrite request url[%s] failed, err: %v", req.Request.RequestURI, err)
		if err := resp.WriteError(http.StatusBadGateway, &metadata.RespError{
			Msg:     errors.New("rewrite request failed"),
			ErrCode: common.CCErrRewriteRequestUriFailed,
			Data:    nil,
		}); err != nil {
			blog.Errorf("response request[url: %s] failed, err: %v", req.Request.RequestURI, err)
			return
		}
		return
	}

	defer func() {
		if err != nil {
			blog.Errorf("proxy request url[%s] failed, err: %v", req.Request.RequestURI, err)
			if rerr := resp.WriteError(http.StatusBadGateway, &metadata.RespError{
				Msg:     errors.New("rewrite request failed"),
				ErrCode: common.CCErrRewriteRequestUriFailed,
				Data:    nil,
			}); rerr != nil {
				blog.Errorf("proxy request[url: %s] failed, err: %v", req.Request.RequestURI, rerr)
				return
			}
			return
		}
	}()

	servers := make([]string, 0)
	switch kind {
	case TopoType:
		servers, err = s.Disc.TopoServer().GetServers()

	case ProcType:
		servers, err = s.Disc.ProcServer().GetServers()

	case EventType:
		servers, err = s.Disc.EventServer().GetServers()

	case HostType:
		servers, err = s.Disc.HostServer().GetServers()

	}

	if err != nil {
		return
	}

	if strings.HasPrefix(servers[0], "https://") {
		req.Request.URL.Host = servers[0][8:]
		req.Request.URL.Scheme = "https"
	}

	req.Request.URL.Host = servers[0][7:]
	req.Request.URL.Scheme = "http"

	chain.ProcessFilter(req, resp)
}
