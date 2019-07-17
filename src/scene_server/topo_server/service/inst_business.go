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

package service

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"configcenter/src/auth"
	authmeta "configcenter/src/auth/meta"
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/condition"
	"configcenter/src/common/mapstr"
	gparams "configcenter/src/common/paraparse"
	"configcenter/src/common/util"
	"configcenter/src/scene_server/topo_server/core/types"
)

// CreateBusiness create a new business
func (s *Service) CreateBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	data.Set(common.BKDefaultField, 0)
	business, err := s.Core.BusinessOperation().CreateBusiness(params, obj, data)
	if err != nil {
		blog.Errorf("create business failed, err: %v", err)
		return nil, err
	}

	businessID, err := business.GetInstID()
	if err != nil {
		blog.Errorf("unexpected error, create business success, but get id failed, err: %+v", err)
		return nil, params.Err.Error(common.CCErrCommParamsInvalid)
	}

	// auth: register business to iam
	if err := s.AuthManager.RegisterBusinessesByID(params.Context, params.Header, businessID); err != nil {
		blog.Errorf("create business success, but register to iam failed, err: %v", err)
		return nil, params.Err.Error(common.CCErrCommRegistResourceToIAMFailed)
	}

	return business, nil
}

// DeleteBusiness delete the business
func (s *Service) DeleteBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {

	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	bizID, err := strconv.ParseInt(pathParams("app_id"), 10, 64)
	if nil != err {
		blog.Errorf("[api-business]failed to parse the biz id, error info is %s", err.Error())
		return nil, params.Err.Errorf(common.CCErrCommParamsNeedInt, "business id")
	}

	// auth: deregister business to iam
	if err := s.AuthManager.DeregisterBusinessesByID(params.Context, params.Header, bizID); err != nil {
		blog.Errorf("delete business failed, deregister business failed, err: %+v", err)
		return nil, params.Err.Errorf(common.CCErrCommUnRegistResourceToIAMFailed)
	}

	return nil, s.Core.BusinessOperation().DeleteBusiness(params, obj, bizID)
}

// UpdateBusiness update the business
func (s *Service) UpdateBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {

	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	bizID, err := strconv.ParseInt(pathParams("app_id"), 10, 64)
	if nil != err {
		blog.Errorf("[api-business]failed to parse the biz id, error info is %s", err.Error())
		return nil, params.Err.Errorf(common.CCErrCommParamsNeedInt, "business id")
	}

	err = s.Core.BusinessOperation().UpdateBusiness(params, data, obj, bizID)
	if err != nil {
		return nil, err
	}

	// auth: update registered business to iam
	if err := s.AuthManager.UpdateRegisteredBusinessByID(params.Context, params.Header, bizID); err != nil {
		blog.Errorf("update business success, but update registered business failed, err: %+v", err)
		return nil, params.Err.Errorf(common.CCErrCommRegistResourceToIAMFailed)
	}

	return nil, nil
}

// UpdateBusinessStatus update the business status
func (s *Service) UpdateBusinessStatus(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {

	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	bizID, err := strconv.ParseInt(pathParams("app_id"), 10, 64)
	if nil != err {
		blog.Errorf("[api-business]failed to parse the biz id, error info is %s", err.Error())
		return nil, params.Err.Errorf(common.CCErrCommParamsNeedInt, "business id")
	}
	data = mapstr.New()
	_, bizs, err := s.Core.BusinessOperation().FindBusiness(params, obj, nil, condition.CreateCondition().Field(common.BKAppIDField).Eq(bizID))
	if nil != err {
		return nil, err
	}
	if len(bizs) <= 0 {
		return nil, params.Err.Error(common.CCErrCommNotFound)
	}
	switch common.DataStatusFlag(pathParams("flag")) {
	case common.DataStatusDisabled:
		innerCond := condition.CreateCondition()
		innerCond.Field(common.BKAsstObjIDField).Eq(obj.Object().ObjectID)
		innerCond.Field(common.BKAsstInstIDField).Eq(bizID)
		if err := s.Core.AssociationOperation().CheckBeAssociation(params, obj, innerCond); nil != err {
			return nil, err
		}

		// check if this business still has hosts.
		has, err := s.Core.BusinessOperation().HasHosts(params, bizID)
		if err != nil {
			return nil, err
		}
		if has {
			return nil, params.Err.Error(common.CCErrTopoArchiveBusinessHasHost)
		}

		data.Set(common.BKDataStatusField, pathParams("flag"))
	case common.DataStatusEnable:
		name, err := bizs[0].GetInstName()
		if nil != err {
			return nil, params.Err.Error(common.CCErrCommNotFound)
		}
		name = name + common.BKDataRecoverSuffix
		if len(name) >= common.FieldTypeSingleLenChar {
			name = name[:common.FieldTypeSingleLenChar]
		}
		data.Set(common.BKAppNameField, name)
		data.Set(common.BKDataStatusField, pathParams("flag"))
	default:
		return nil, params.Err.Errorf(common.CCErrCommParamsIsInvalid, pathParams("flag"))
	}

	err = s.Core.BusinessOperation().UpdateBusiness(params, data, obj, bizID)
	if err != nil {
		blog.Errorf("UpdateBusinessStatus failed, run update failed, err: %+v", err)
		return nil, err
	}
	if err := s.AuthManager.UpdateRegisteredBusinessByID(params.Context, params.Header, bizID); err != nil {
		blog.Errorf("UpdateBusinessStatus failed, update register business info failed, err: %+v", err)
		return nil, params.Err.Error(common.CCErrCommRegistResourceToIAMFailed)
	}
	return nil, nil
}

// find business list with these info：
// 1. have any authorized resources in a business.
// 2. only returned with a few field for this business info.
func (s *Service) SearchReducedBusinessList(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}
	fields := []string{common.BKAppIDField, common.BKAppNameField, "business_dept_id", "business_dept_name"}
	cond := condition.CreateCondition()
	cond.Field(common.BKDataStatusField).NotEq(common.DataStatusDisabled)
	cond.Field(common.BKDefaultField).Eq(0)
	if s.AuthManager.Enabled() {
		user := authmeta.UserInfo{UserName: params.User, SupplierAccount: params.SupplierAccount}
		appList, err := s.AuthManager.Authorize.GetAnyAuthorizedBusinessList(params.Context, user)
		if err != nil {
			return nil, params.Err.Error(common.CCErrorTopoGetAuthorizedBusinessListFailed)
		}

		// sort for prepare to find business with page.
		sort.Sort(util.Int64Slice(appList))
		// user can only find business that is already authorized.
		cond.Field(common.BKAppIDField).In(appList)

	}

	cnt, instItems, err := s.Core.BusinessOperation().FindBusiness(params, obj, fields, cond)
	if nil != err {
		blog.Errorf("[api-business] failed to find the objects(%s), error info is %s", pathParams("obj_id"), err.Error())
		return nil, err
	}

	datas := make([]mapstr.MapStr, 0)
	for _, item := range instItems {
		instMap := item.GetValues()
		inst := mapstr.New()
		inst[common.BKAppIDField] = instMap[common.BKAppIDField]
		inst[common.BKAppNameField] = instMap[common.BKAppNameField]
		inst["business_dept_id"] = instMap["business_dept_id"]
		inst["business_dept_name"] = instMap["business_dept_name"]

		if val, exist := instMap["business_dept_id"]; exist {
			inst["business_dept_id"] = val
		} else {
			inst["business_dept_id"] = ""
		}
		if val, exist := instMap["business_dept_name"]; exist {
			inst["business_dept_name"] = val
		} else {
			inst["business_dept_name"] = ""
		}
		datas = append(datas, inst)
	}

	result := mapstr.MapStr{}
	result.Set("count", cnt)
	result.Set("info", datas)
	return result, nil
}

// SearchBusiness search the business by condition
func (s *Service) SearchBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	searchCond := &gparams.SearchParams{
		Condition: mapstr.New(),
	}
	if err := data.MarshalJSONInto(&searchCond); nil != err {
		blog.Errorf("failed to parse the params, error info is %s", err.Error())
		return nil, params.Err.New(common.CCErrCommParamsInvalid, err.Error())
	}

	innerCond := condition.CreateCondition()
	switch searchCond.Native {
	case 1: // native mode
		if err := innerCond.Parse(searchCond.Condition); nil != err {
			blog.Errorf("[api-biz] failed to parse the input data, error info is %s", err.Error())
			return nil, params.Err.Error(common.CCErrTopoAppSearchFailed)
		}
	default:
		if err := innerCond.Parse(gparams.ParseAppSearchParams(searchCond.Condition)); nil != err {
			blog.Errorf("[api-biz] failed to parse the input data, error info is %s", err.Error())
			return nil, params.Err.Error(common.CCErrTopoAppSearchFailed)
		}
	}

	// parse business id from user's condition for testing.
	var bizIDs []int64
	biz, exist := searchCond.Condition[common.BKAppIDField]
	if exist {
		// constrict that bk_biz_id field can only be a numeric value,
		// operators like or/in/and is not allowed.
		if bizcond, ok := biz.(map[string]interface{}); ok {
			if cond, ok := bizcond["$eq"]; ok {
				if reflect.TypeOf(cond).ConvertibleTo(reflect.TypeOf(int64(1))) == false {
					return nil, params.Err.Errorf(common.CCErrCommParamsInvalid, common.BKAppIDField)
				}
				bizIDs = []int64{int64(cond.(float64))}
			}
			if cond, ok := bizcond["$in"]; ok {
				if conds, ok := cond.([]interface{}); ok {
					for _, c := range conds {
						if reflect.TypeOf(c).ConvertibleTo(reflect.TypeOf(int64(1))) == false {
							return nil, params.Err.Errorf(common.CCErrCommParamsInvalid, common.BKAppIDField)
						}
						bizIDs = append(bizIDs, int64(c.(float64)))
					}
				}
			}
		} else if reflect.TypeOf(biz).ConvertibleTo(reflect.TypeOf(int64(1))) {
			bizIDs = []int64{int64(searchCond.Condition[common.BKAppIDField].(float64))}
		} else {
			return nil, params.Err.New(common.CCErrCommParamsInvalid, common.BKAppIDField)
		}
	}

	if s.AuthManager.Enabled() {
		user := authmeta.UserInfo{UserName: params.User, SupplierAccount: params.SupplierAccount}
		appList, err := s.AuthManager.Authorize.GetExactAuthorizedBusinessList(params.Context, user)
		if err != nil {
			return nil, params.Err.Error(common.CCErrorTopoGetAuthorizedBusinessListFailed)
		}

		if len(bizIDs) > 0 {
			// this means that user want to find a specific business.
			// now we check if he has this authority.
			for _, bizID := range bizIDs {
				if !util.InArray(bizID, appList) {
					noAuthResp, err := s.AuthManager.GenBusinessAuditNoPermissionResp(params.Context, params.Header, bizID)
					if err != nil {
						return nil, params.Err.Error(common.CCErrTopoAppSearchFailed)
					}
					return noAuthResp, auth.NoAuthorizeError
				}
			}
			// now you have the authority.
		} else {
			// sort for prepare to find business with page.
			sort.Sort(util.Int64Slice(appList))
			// user can only find business that is already authorized.
			innerCond.Field(common.BKAppIDField).In(appList)
		}
	}

	if _, ok := searchCond.Condition[common.BKDataStatusField]; !ok {
		innerCond.Field(common.BKDataStatusField).NotEq(common.DataStatusDisabled)
	}

	innerCond.Field(common.BKDefaultField).Eq(0)
	innerCond.SetPage(searchCond.Page)
	innerCond.SetFields(searchCond.Fields)

	cnt, instItems, err := s.Core.BusinessOperation().FindBusiness(params, obj, searchCond.Fields, innerCond)
	if nil != err {
		blog.Errorf("[api-business] failed to find the objects(%s), error info is %s", pathParams("obj_id"), err.Error())
		return nil, err
	}

	result := mapstr.MapStr{}
	result.Set("count", cnt)
	result.Set("info", instItems)

	return result, nil
}

// search archived business by condition
func (s *Service) SearchArchivedBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {

	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	innerCond := condition.CreateCondition()
	if err = innerCond.Parse(data); nil != err {
		blog.Errorf("[api-biz] failed to parse the input data, error info is %s", err.Error())
		return nil, params.Err.New(common.CCErrTopoAppSearchFailed, err.Error())
	}
	innerCond.Field(common.BKDefaultField).Eq(common.DefaultAppFlag)

	if s.AuthManager.Enabled() {
		user := authmeta.UserInfo{UserName: params.User, SupplierAccount: params.SupplierAccount}
		appList, err := s.AuthManager.Authorize.GetExactAuthorizedBusinessList(params.Context, user)
		if err != nil {
			return nil, params.Err.Error(common.CCErrorTopoGetAuthorizedBusinessListFailed)
		}
		// sort for prepare to find business with page.
		sort.Sort(util.Int64Slice(appList))
		// user can only find business that is already authorized.
		innerCond.Field(common.BKAppIDField).In(appList)
	}

	cnt, instItems, err := s.Core.BusinessOperation().FindBusiness(params, obj, []string{}, innerCond)
	if nil != err {
		blog.Errorf("[api-business] failed to find the objects(%s), error info is %s", pathParams("obj_id"), err.Error())
		return nil, err
	}
	result := mapstr.MapStr{}
	result.Set("count", cnt)
	result.Set("info", instItems)
	return result, nil
}

// CreateDefaultBusiness create the default business
func (s *Service) CreateDefaultBusiness(params types.ContextParams, pathParams, queryParams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}

	data.Set(common.BKDefaultField, common.DefaultAppFlag)
	business, err := s.Core.BusinessOperation().CreateBusiness(params, obj, data)
	if err != nil {
		return nil, fmt.Errorf("create business failed, err: %+v", err)
	}

	businessID, err := business.GetInstID()
	if err != nil {
		return nil, fmt.Errorf("unexpected error, create default business success, but get id failed, err: %+v", err)
	}

	// auth: register business to iam
	if err := s.AuthManager.RegisterBusinessesByID(params.Context, params.Header, businessID); err != nil {
		blog.Errorf("create default business failed, register business failed, err: %+v", err)
		return nil, params.Err.Error(common.CCErrCommRegistResourceToIAMFailed)
	}

	return business, nil
}

func (s *Service) GetInternalModule(params types.ContextParams, pathParams, queryparams ParamsGetter, data mapstr.MapStr) (interface{}, error) {
	obj, err := s.Core.ObjectOperation().FindSingleObject(params, common.BKInnerObjIDApp)
	if nil != err {
		blog.Errorf("failed to search the business, %s", err.Error())
		return nil, err
	}
	bizID, err := strconv.ParseInt(pathParams("app_id"), 10, 64)
	if nil != err {
		return nil, params.Err.New(common.CCErrTopoAppSearchFailed, err.Error())
	}

	_, result, err := s.Core.BusinessOperation().GetInternalModule(params, obj, bizID)
	if nil != err {
		return nil, err
	}

	return result, nil
}
