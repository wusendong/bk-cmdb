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

package operation

import (
	"configcenter/src/common/metadata"
	"configcenter/src/scene_server/topo_server/core/model"
)

type opcondition struct {
	InstID []int64 `json:"inst_ids"`
}

type deleteCondition struct {
	opcondition `json:",inline"`
}

type updateCondition struct {
	InstID   int64                  `json:"inst_id"`
	InstInfo map[string]interface{} `json:"datas"`
}

// OpCondition the condition operation
type OpCondition struct {
	Delete deleteCondition   `json:"delete"`
	Update []updateCondition `json:"update"`
}

type instBatchInfo struct {
	// BatchInfo batch info
	BatchInfo *map[int]map[string]interface{} `json:"BatchInfo"`
	InputType string                          `json:"input_type"`
}
type instNameAsst struct {
	ID         string                 `json:"id"`
	ObjID      string                 `json:"bk_obj_id"`
	ObjIcon    string                 `json:"bk_obj_icon"`
	InstID     int64                  `json:"bk_inst_id"`
	ObjectName string                 `json:"bk_obj_name"`
	InstName   string                 `json:"bk_inst_name"`
	InstInfo   map[string]interface{} `json:"inst_info,omitempty"`
}

// ConditionItem subcondition
type ConditionItem struct {
	Field    string      `json:"field,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

// AssociationParams  association params
type AssociationParams struct {
	Page      metadata.BasePage          `json:"page,omitempty"`
	Fields    map[string][]string        `json:"fields,omitempty"`
	Condition map[string][]ConditionItem `json:"condition,omitempty"`
}

// commonInstTopo common inst topo
type commonInstTopo struct {
	instNameAsst
	Count    int            `json:"count"`
	Children []instNameAsst `json:"children"`
}

type commonInstTopoV2 struct {
	Prev interface{} `json:"prev"`
	Next interface{} `json:"next"`
	Curr interface{} `json:"curr"`
}

type deletedInst struct {
	instID int64
	obj    model.Object
}
