// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tasks

import (
	"context"
	"database/sql"
	"fmt"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"

	"yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/compute/models"
)

type DiskCleanOverduedSnapshots struct {
	SDiskBaseTask
}

func init() {
	taskman.RegisterTask(SnapshotCleanupTask{})
}

type SnapshotCleanupTask struct {
	taskman.STask
}

func (self *SnapshotCleanupTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	now, err := self.Params.GetTime("tick")
	if err != nil {
		self.SetStageFailed(ctx, jsonutils.NewString("failed get tick"))
		return
	}
	var snapshots = make([]models.SSnapshot, 0)
	err = models.SnapshotManager.Query().
		Equals("fake_deleted", false).
		Equals("created_by", compute.SNAPSHOT_AUTO).
		LE("expired_at", now).All(&snapshots)
	if err == sql.ErrNoRows {
		self.SetStageComplete(ctx, nil)
		return
	} else if err != nil {
		self.SetStageFailed(ctx, jsonutils.NewString(fmt.Sprintf("failed get snapshot %s", err)))
		return
	}
	self.StartSnapshotsDelete(ctx, snapshots)
}

func (self *SnapshotCleanupTask) OnInitFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	log.Errorf("delete snapshots failed %s", data)
	self.OnInit(ctx, obj, data)
}

func (self *SnapshotCleanupTask) StartSnapshotsDelete(ctx context.Context, snapshots []models.SSnapshot) {
	snapshot := snapshots[0]
	snapshots = snapshots[1:]
	if len(snapshots) > 0 {
		self.Params.Set("snapshots", jsonutils.Marshal(snapshots))
	}
	self.SetStage("OnDeleteSnapshot", nil)
	snapshot.SetModelManager(models.SnapshotManager, &snapshot)
	err := snapshot.StartSnapshotDeleteTask(ctx, self.UserCred, false, self.GetId(), 0, 0)
	if err != nil {
		self.OnDeleteSnapshotFailed(ctx, self.GetObject(), nil)
	}
}

func (self *SnapshotCleanupTask) OnDeleteSnapshot(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	var snapshots = make([]models.SSnapshot, 0)
	err := self.Params.Unmarshal(&snapshots, "snapshots")
	if err != nil {
		self.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
		return
	}
	if len(snapshots) > 0 {
		self.StartSnapshotsDelete(ctx, snapshots)
	} else {
		self.SetStageComplete(ctx, nil)
	}
}

func (self *SnapshotCleanupTask) OnDeleteSnapshotFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	log.Errorf("snapshot delete faield %s", data)
	self.SetStageFailed(ctx, data)
}
