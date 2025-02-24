// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package topo

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/log"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"

	"github.com/pingcap/tidb-dashboard/util/distro"
	"github.com/pingcap/tidb-dashboard/util/netutil"
)

const tidbTopologyKeyPrefix = "/topology/tidb/"

func GetTiDBInstances(ctx context.Context, etcdClient *clientv3.Client) ([]TiDBInfo, error) {
	resp, err := etcdClient.Get(ctx, tidbTopologyKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, ErrEtcdRequestFailed.Wrap(err, "Failed to read topology from etcd key `%s`", tidbTopologyKeyPrefix)
	}

	nodesAlive := make(map[string]struct{}, len(resp.Kvs))
	nodesInfo := make(map[string]*TiDBInfo, len(resp.Kvs))

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.HasPrefix(key, tidbTopologyKeyPrefix) {
			continue
		}
		// remainingKey looks like `ip:port/info` or `ip:port/ttl`.
		remainingKey := key[len(tidbTopologyKeyPrefix):]
		keyParts := strings.Split(remainingKey, "/")
		if len(keyParts) != 2 {
			log.Warn("Ignored invalid topology key",
				zap.String("component", distro.R().TiDB),
				zap.String("key", key))
			continue
		}

		switch keyParts[1] {
		case "info":
			node, err := parseTiDBInfo(keyParts[0], kv.Value)
			if err == nil {
				nodesInfo[keyParts[0]] = node
			} else {
				log.Warn("Ignored invalid topology info entry",
					zap.String("component", distro.R().TiDB),
					zap.String("key", key),
					zap.String("value", string(kv.Value)),
					zap.Error(err))
			}
		case "ttl":
			alive, err := parseTiDBAliveness(kv.Value)
			if err == nil {
				nodesAlive[keyParts[0]] = struct{}{}
				if !alive {
					log.Warn("Component alive TTL has expired (maybe local time are not synchronized)",
						zap.String("component", distro.R().TiDB),
						zap.String("key", key),
						zap.String("value", string(kv.Value)))
				}
			} else {
				log.Warn("Ignored invalid topology TTL entry",
					zap.String("component", distro.R().TiDB),
					zap.String("key", key),
					zap.String("value", string(kv.Value)),
					zap.Error(err))
			}
		}
	}

	nodes := make([]TiDBInfo, 0)

	for addr, info := range nodesInfo {
		if _, ok := nodesAlive[addr]; ok {
			info.Status = ComponentStatusUp
		}
		nodes = append(nodes, *info)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IP < nodes[j].IP {
			return true
		}
		if nodes[i].IP > nodes[j].IP {
			return false
		}
		return nodes[i].Port < nodes[j].Port
	})

	return nodes, nil
}

func parseTiDBInfo(address string, value []byte) (*TiDBInfo, error) {
	ds := struct {
		Version        string `json:"version"`
		GitHash        string `json:"git_hash"`
		StatusPort     uint   `json:"status_port"`
		DeployPath     string `json:"deploy_path"`
		StartTimestamp int64  `json:"start_timestamp"`
	}{}

	err := json.Unmarshal(value, &ds)
	if err != nil {
		return nil, ErrInvalidTopologyData.Wrap(err, "Read topology value failed")
	}
	hostname, port, err := netutil.ParseHostAndPortFromAddress(address)
	if err != nil {
		return nil, ErrInvalidTopologyData.Wrap(err, "Read topology address failed")
	}

	return &TiDBInfo{
		GitHash:        ds.GitHash,
		Version:        ds.Version,
		IP:             hostname,
		Port:           port,
		DeployPath:     ds.DeployPath,
		Status:         ComponentStatusUnreachable,
		StatusPort:     ds.StatusPort,
		StartTimestamp: ds.StartTimestamp,
	}, nil
}

func parseTiDBAliveness(value []byte) (bool, error) {
	unixTimestampNano, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		return false, ErrInvalidTopologyData.Wrap(err, "Parse topology TTL info failed")
	}
	t := time.Unix(0, int64(unixTimestampNano))
	if time.Since(t) > time.Second*45 {
		return false, nil
	}
	return true, nil
}
