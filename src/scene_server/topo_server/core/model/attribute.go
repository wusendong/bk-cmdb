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

package model

import (
	"context"
	"encoding/json"
	"io"

	"configcenter/src/common/util"

	"configcenter/src/apimachinery"
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/condition"
	frtypes "configcenter/src/common/mapstr"
	metadata "configcenter/src/common/metadata"
	"configcenter/src/scene_server/topo_server/core/types"
)

// Attribute attribute opeartion interface declaration
type Attribute interface {
	Operation
	Parse(data frtypes.MapStr) (*metadata.Attribute, error)

	Origin() metadata.Attribute

	IsAssociationType() bool

	SetSupplierAccount(supplierAccount string)
	GetSupplierAccount() string

	GetParentObject() (Object, error)
	GetChildObject() (Object, error)

	SetParentObject(objID string) error
	SetChildObject(objID string) error

	SetObjectID(objectID string)
	GetObjectID() string

	SetID(attributeID string)
	GetID() string

	SetName(attributeName string)
	GetName() string

	SetGroup(grp Group)
	GetGroup() (Group, error)

	SetGroupIndex(attGroupIndex int64)
	GetGroupIndex() int64

	SetUnint(unit string)
	GetUnint() string

	SetPlaceholder(placeHolder string)
	GetPlaceholder() string

	SetIsEditable(isEditable bool)
	GetIsEditable() bool

	SetIsPre(isPre bool)
	GetIsPre() bool

	SetIsReadOnly(isReadOnly bool)
	GetIsReadOnly() bool

	SetIsOnly(isOnly bool)
	GetIsOnly() bool

	SetIsRequired(isRequired bool)
	GetIsRequired() bool

	SetIsSystem(isSystem bool)
	GetIsSystem() bool

	SetIsAPI(isAPI bool)
	GetIsAPI() bool

	SetType(attributeType string)
	GetType() string

	SetOption(attributeOption interface{})
	GetOption() interface{}

	SetDescription(attributeDescription string)
	GetDescription() string

	SetCreator(attributeCreator string)
	GetCreator() string

	ToMapStr() (frtypes.MapStr, error)
}

var _ Attribute = (*attribute)(nil)

// attribute the metadata structure definition of the model attribute
type attribute struct {
	attr      metadata.Attribute
	isNew     bool
	params    types.ContextParams
	clientSet apimachinery.ClientSetInterface
}

func (a *attribute) Origin() metadata.Attribute {
	return a.attr
}

func (a *attribute) IsAssociationType() bool {
	return util.IsAssocateProperty(a.attr.PropertyType)
}

func (a *attribute) searchObjects(objID string) ([]metadata.Object, error) {
	cond := condition.CreateCondition()
	cond.Field(common.BKOwnerIDField).Eq(a.params.SupplierAccount).Field(common.BKObjIDField).Eq(objID)

	condStr, err := cond.ToMapStr().ToJSON()
	if nil != err {
		return nil, err
	}
	rsp, err := a.clientSet.ObjectController().Meta().SelectObjects(context.Background(), a.params.Header, condStr)

	if nil != err {
		blog.Errorf("failed to request the object controller, error info is %s", err.Error())
		return nil, a.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to search the object(%s), error info is %s", objID, rsp.ErrMsg)
		return nil, a.params.Err.Error(rsp.Code)
	}

	return rsp.Data, nil

}

func (a *attribute) GetParentObject() (Object, error) {

	cond := condition.CreateCondition()
	cond.Field(metadata.AssociationFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AssociationFieldObjectID).Eq(a.attr.ObjectID)
	cond.Field(metadata.AssociationFieldObjectAttributeID).Eq(a.attr.PropertyID)

	rsp, err := a.clientSet.ObjectController().Meta().SelectObjectAssociations(context.Background(), a.params.Header, cond.ToMapStr())
	if nil != err {
		blog.Errorf("[model-obj] failed to request the object controller, error info is %s", err.Error())
		return nil, err
	}

	for _, asst := range rsp.Data {

		rspRst, err := a.searchObjects(asst.ObjectID)
		if nil != err {
			blog.Errorf("[model-obj] failed to search the object(%s)'s parent, error info is %s", asst.ObjectID, err.Error())
			return nil, err
		}

		objItems := CreateObject(a.params, a.clientSet, rspRst)
		for _, item := range objItems { // only one object
			return item, nil
		}

	}

	return nil, io.EOF
}
func (a *attribute) GetChildObject() (Object, error) {

	cond := condition.CreateCondition()
	cond.Field(metadata.AssociationFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AssociationFieldAssociationObjectID).Eq(a.attr.ObjectID)
	cond.Field(metadata.AssociationFieldObjectAttributeID).Eq(a.attr.PropertyID)

	rsp, err := a.clientSet.ObjectController().Meta().SelectObjectAssociations(context.Background(), a.params.Header, cond.ToMapStr())
	if nil != err {
		blog.Errorf("[model-obj] failed to request the object controller, error info is %s", err.Error())
		return nil, err
	}

	for _, asst := range rsp.Data {

		rspRst, err := a.searchObjects(asst.ObjectID)
		if nil != err {
			blog.Errorf("[model-obj] failed to search the object(%s)'s child, error info is %s", asst.ObjectID, err.Error())
			return nil, err
		}

		objItems := CreateObject(a.params, a.clientSet, rspRst)
		for _, item := range objItems { // only one object
			return item, nil
		}

	}

	return nil, io.EOF
}

func (a *attribute) SetParentObject(objID string) error {

	cond := condition.CreateCondition()
	cond.Field(metadata.AssociationFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AssociationFieldObjectAttributeID).Eq(a.attr.PropertyID)
	cond.Field(metadata.AssociationFieldObjectID).Eq(a.attr.ObjectID)

	rsp, err := a.clientSet.ObjectController().Meta().SelectObjectAssociations(context.Background(), a.params.Header, cond.ToMapStr())
	if nil != err {
		blog.Errorf("[model-attr] failed to request the object controller, error info is %s", err.Error())
		return a.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("[model-attr] failed to search the parent association, error info is %s", rsp.ErrMsg)
		return a.params.Err.Error(rsp.Code)
	}

	// create
	if 0 == len(rsp.Data) {

		asst := &metadata.Association{}
		asst.OwnerID = a.params.SupplierAccount
		asst.ObjectAttID = a.attr.PropertyID
		asst.AsstObjID = objID
		asst.ObjectID = a.attr.ObjectID

		rsp, err := a.clientSet.ObjectController().Meta().CreateObjectAssociation(context.Background(), a.params.Header, asst)

		if nil != err {
			blog.Errorf("[model-obj] failed to request the object controller, error info is %s", err.Error())
			return a.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
		}

		if common.CCSuccess != rsp.Code {
			blog.Errorf("[model-obj] failed to set the main line association parent, error info is %s", rsp.ErrMsg)
			return a.params.Err.Error(rsp.Code)
		}

		return nil
	}

	// update
	for _, asst := range rsp.Data {

		asst.AsstObjID = objID

		rsp, err := a.clientSet.ObjectController().Meta().UpdateObjectAssociation(context.Background(), asst.ID, a.params.Header, nil)
		if nil != err {
			blog.Errorf("[model-obj] failed to request object controller, error info is %s", err.Error())
			return err
		}

		if common.CCSuccess != rsp.Code {
			blog.Errorf("[model-obj] failed to update the child association, error info is %s", rsp.ErrMsg)
			return a.params.Err.Error(rsp.Code)
		}
	}

	return nil
}
func (a *attribute) SetChildObject(objID string) error {

	cond := condition.CreateCondition()
	cond.Field(metadata.AssociationFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AssociationFieldObjectAttributeID).Eq(a.attr.PropertyID)
	cond.Field(metadata.AssociationFieldAssociationObjectID).Eq(a.attr.ObjectID)

	rsp, err := a.clientSet.ObjectController().Meta().SelectObjectAssociations(context.Background(), a.params.Header, cond.ToMapStr())
	if nil != err {
		blog.Errorf("[model-attr] failed to request the object controller, error info is %s", err.Error())
		return a.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("[model-attr] failed to search the child association, error info is %s", rsp.ErrMsg)
		return a.params.Err.Error(rsp.Code)
	}

	// create
	if 0 == len(rsp.Data) {

		asst := &metadata.Association{}
		asst.OwnerID = a.params.SupplierAccount
		asst.ObjectAttID = a.attr.PropertyID
		asst.AsstObjID = a.attr.ObjectID
		asst.ObjectID = objID

		rsp, err := a.clientSet.ObjectController().Meta().CreateObjectAssociation(context.Background(), a.params.Header, asst)

		if nil != err {
			blog.Errorf("[model-obj] failed to request the object controller, error info is %s", err.Error())
			return a.params.Err.Error(common.CCErrCommHTTPDoRequestFailed)
		}

		if common.CCSuccess != rsp.Code {
			blog.Errorf("[model-obj] failed to set the main line association parent, error info is %s", rsp.ErrMsg)
			return a.params.Err.Error(rsp.Code)
		}

		return nil
	}

	// update
	for _, asst := range rsp.Data {

		asst.ObjectID = objID

		rsp, err := a.clientSet.ObjectController().Meta().UpdateObjectAssociation(context.Background(), asst.ID, a.params.Header, nil)
		if nil != err {
			blog.Errorf("[model-obj] failed to request object controller, error info is %s", err.Error())
			return err
		}

		if common.CCSuccess != rsp.Code {
			blog.Errorf("[model-obj] failed to update the child association, error info is %s", rsp.ErrMsg)
			return a.params.Err.Error(rsp.Code)
		}
	}

	return nil
}

func (a *attribute) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.attr)
}

func (a *attribute) Parse(data frtypes.MapStr) (*metadata.Attribute, error) {
	attr, err := a.attr.Parse(data)
	if nil != err {
		return attr, err
	}
	if a.attr.IsOnly {
		a.attr.IsRequired = true
	}
	return nil, err
}

func (a *attribute) ToMapStr() (frtypes.MapStr, error) {

	rst := metadata.SetValueToMapStrByTags(&a.attr)
	return rst, nil

}

func (a *attribute) Create() error {

	// check the property id repeated
	cond := condition.CreateCondition()
	cond.Field(metadata.AttributeFieldPropertyID).Eq(a.attr.PropertyID)
	cond.Field(metadata.AttributeFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AttributeFieldObjectID).Eq(a.attr.ObjectID)
	attrItems, err := a.search(cond)
	if nil != err {
		blog.Errorf("[model-attr] failed to check the property id (%s), error info is %s", a.attr.PropertyID, err.Error())
		return err
	}

	if 0 != len(attrItems) {
		blog.Errorf("[model-attr] the property id(%s) is repeated", a.attr.PropertyID)
		return a.params.Err.Error(common.CCErrCommDuplicateItem)
	}

	// check the property name repeated

	cond = condition.CreateCondition()
	cond.Field(metadata.AttributeFieldPropertyName).Eq(a.attr.PropertyName)
	cond.Field(metadata.AttributeFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AttributeFieldObjectID).Eq(a.attr.ObjectID)
	attrItems, err = a.search(cond)
	if nil != err {
		blog.Errorf("[model-attr] failed to check the property name (%s), error info is %s", a.attr.PropertyName, err.Error())
		return err
	}

	if 0 != len(attrItems) {
		blog.Errorf("[model-attr] the property name(%s) is repeated", a.attr.PropertyName)
		return a.params.Err.Error(common.CCErrCommDuplicateItem)
	}

	// create a new record
	rsp, err := a.clientSet.ObjectController().Meta().CreateObjectAtt(context.Background(), a.params.Header, &a.attr)

	if nil != err {
		blog.Errorf("faield to request the object controller, the error info is %s", err.Error())
		return err
	}

	if common.CCSuccess != rsp.Code {
		return err
	}

	a.attr.ID = rsp.Data.ID

	return nil
}

func (a *attribute) Update(data frtypes.MapStr) error {

	rsp, err := a.clientSet.ObjectController().Meta().UpdateObjectAttByID(context.Background(), a.attr.ID, a.params.Header, a.attr.ToMapStr())

	if nil != err {
		blog.Errorf("failed to request object controller, error info is %s", err.Error())
		return err
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to update the object attribute(%s), error info is %s", a.attr.PropertyID, rsp.ErrMsg)
		return a.params.Err.Error(common.CCErrTopoObjectAttributeUpdateFailed)
	}

	return nil
}
func (a *attribute) search(cond condition.Condition) ([]metadata.Attribute, error) {

	rsp, err := a.clientSet.ObjectController().Meta().SelectObjectAttWithParams(context.Background(), a.params.Header, cond.ToMapStr())

	if nil != err {
		blog.Errorf("failed to request to object controller, error info is %s", err.Error())
		return nil, err
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to query the object controller, error info is %s", err.Error())
		return nil, a.params.Err.Error(common.CCErrTopoObjectAttributeSelectFailed)
	}

	return rsp.Data, nil
}
func (a *attribute) IsExists() (bool, error) {

	cond := condition.CreateCondition()
	cond.Field(common.BKOwnerIDField).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AttributeFieldObjectID).Eq(a.attr.ObjectID)
	cond.Field(metadata.AttributeFieldPropertyID).Eq(a.attr.PropertyID)

	items, err := a.search(cond)
	if nil != err {
		return false, err
	}

	return 0 != len(items), nil
}

func (a *attribute) Delete() error {

	cond := condition.CreateCondition()
	cond.Field(metadata.AttributeFieldObjectID).Eq(a.attr.ObjectID)
	cond.Field(metadata.AttributeFieldSupplierAccount).Eq(a.params.SupplierAccount)
	cond.Field(metadata.AttributeFieldPropertyID).Eq(a.attr.PropertyID)

	rsp, err := a.clientSet.ObjectController().Meta().DeleteObjectAttByID(context.Background(), a.attr.ID, a.params.Header, cond.ToMapStr())

	if nil != err {
		blog.Errorf("failed to request object, error info is %s", err.Error())
		return err
	}

	if common.CCSuccess != rsp.Code {
		blog.Errorf("failed to delete attribute,error info is is %s", rsp.ErrMsg)
		return a.params.Err.Error(common.CCErrTopoObjectAttributeDeleteFailed)
	}

	return nil
}

func (a *attribute) Save() error {

	if exists, err := a.IsExists(); nil != err {
		return err
	} else if !exists {
		return a.Create()
	}
	data := metadata.SetValueToMapStrByTags(a.attr)
	return a.Update(data)
}

func (a *attribute) SetSupplierAccount(supplierAccount string) {

	a.attr.OwnerID = supplierAccount
}

func (a *attribute) GetSupplierAccount() string {
	return a.attr.OwnerID
}

func (a *attribute) SetObjectID(objectID string) {
	a.attr.ObjectID = objectID
}

func (a *attribute) GetObjectID() string {
	return a.attr.ObjectID
}

func (a *attribute) SetID(attributeID string) {
	a.attr.PropertyID = attributeID
}

func (a *attribute) GetID() string {
	return a.attr.PropertyID
}

func (a *attribute) SetName(attributeName string) {
	a.attr.PropertyName = attributeName
}

func (a *attribute) GetName() string {
	return a.attr.PropertyName
}

func (a *attribute) SetGroup(grp Group) {
	a.attr.PropertyGroup = grp.GetID()
}

func (a *attribute) GetGroup() (Group, error) {
	return nil, nil
}

func (a *attribute) SetGroupIndex(attGroupIndex int64) {
	a.attr.PropertyIndex = attGroupIndex
}

func (a *attribute) GetGroupIndex() int64 {
	return a.attr.PropertyIndex
}

func (a *attribute) SetUnint(unit string) {
	a.attr.Unit = unit
}

func (a *attribute) GetUnint() string {
	return a.attr.Unit
}

func (a *attribute) SetPlaceholder(placeHolder string) {
	a.attr.Placeholder = placeHolder
}

func (a *attribute) GetPlaceholder() string {
	return a.attr.Placeholder
}

func (a *attribute) SetIsRequired(isRequired bool) {
	a.attr.IsRequired = isRequired
}
func (a *attribute) GetIsRequired() bool {
	return a.attr.IsRequired
}
func (a *attribute) SetIsEditable(isEditable bool) {
	a.attr.IsEditable = isEditable
}

func (a *attribute) GetIsEditable() bool {
	return a.attr.IsEditable
}

func (a *attribute) SetIsPre(isPre bool) {
	a.attr.IsPre = isPre
}

func (a *attribute) GetIsPre() bool {
	return a.attr.IsPre
}

func (a *attribute) SetIsReadOnly(isReadOnly bool) {
	a.attr.IsReadOnly = isReadOnly
}

func (a *attribute) GetIsReadOnly() bool {
	return a.attr.IsReadOnly
}

func (a *attribute) SetIsOnly(isOnly bool) {
	a.attr.IsOnly = isOnly
}

func (a *attribute) GetIsOnly() bool {
	return a.attr.IsOnly
}

func (a *attribute) SetIsSystem(isSystem bool) {
	a.attr.IsSystem = isSystem
}

func (a *attribute) GetIsSystem() bool {
	return a.attr.IsSystem
}

func (a *attribute) SetIsAPI(isAPI bool) {
	a.attr.IsAPI = isAPI
}

func (a *attribute) GetIsAPI() bool {
	return a.attr.IsAPI
}

func (a *attribute) SetType(attributeType string) {
	a.attr.PropertyType = attributeType
}

func (a *attribute) GetType() string {
	return a.attr.PropertyType
}

func (a *attribute) SetOption(attributeOption interface{}) {
	a.attr.Option = attributeOption
}

func (a *attribute) GetOption() interface{} {
	return a.attr.Option
}

func (a *attribute) SetDescription(attributeDescription string) {
	a.attr.Description = attributeDescription
}

func (a *attribute) GetDescription() string {
	return a.attr.Description
}

func (a *attribute) SetCreator(attributeCreator string) {
	a.attr.Creator = attributeCreator
}

func (a *attribute) GetCreator() string {
	return a.attr.Creator
}
