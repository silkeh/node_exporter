// Copyright 2019 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nobtrfs

package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs/btrfs"
)

// A btrfsCollector is a Collector which gathers metrics from Btrfs filesystems.
type btrfsCollector struct {
	fs btrfs.FS
}

func init() {
	registerCollector("btrfs", defaultEnabled, NewBtrfsCollector)
}

// NewBtrfsCollector returns a new Collector exposing XFS statistics.
func NewBtrfsCollector() (Collector, error) {
	fs, err := btrfs.NewFS(*sysPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sysfs: %v", err)
	}

	return &btrfsCollector{
		fs: fs,
	}, nil
}

// Update implements Collector.
func (c *btrfsCollector) Update(ch chan<- prometheus.Metric) error {
	stats, err := c.fs.Stats()
	if err != nil {
		return fmt.Errorf("failed to retrieve Btrfs stats: %v", err)
	}

	for _, s := range stats {
		c.updateBtrfsStats(ch, s)
	}

	return nil
}

type btrfsMetric struct {
	name  string
	desc  string
	value float64
	//metricType      prometheus.ValueType
	extraLabel      []string
	extraLabelValue []string
}

// UpdateBtrfsStats collects statistics for one bcache ID.
func (c *btrfsCollector) updateBtrfsStats(ch chan<- prometheus.Metric, s *btrfs.Stats) {
	const (
		subsystem = "btrfs"
	)

	devLabels := []string{"label", "uuid"}
	metrics := []btrfsMetric{
		{
			name:  "device_count",
			desc:  "Number of devices that are part of the filesystem.",
			value: float64(len(s.Devices)),
		},
		{
			name:  "global_rsv_size_bytes",
			desc:  "Size of global reserve.",
			value: float64(s.Allocation.GlobalRsvSize),
		},
	}

	for n, dev := range s.Devices {
		metrics = append(metrics, []btrfsMetric{
			{
				name:  "device_size",
				desc:  "Size of a device that is part of the filesystem.",
				value: float64(dev.Size),
				extraLabel:      []string{"device"},
				extraLabelValue: []string{n},
			},
		}...)
	}

	metrics = append(metrics, c.getAllocationStats("data", s.Allocation.Data)...)
	metrics = append(metrics, c.getAllocationStats("metadata", s.Allocation.Metadata)...)
	metrics = append(metrics, c.getAllocationStats("system", s.Allocation.System)...)

	for _, m := range metrics {
		labels := append(devLabels, m.extraLabel...)

		desc := prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, m.name),
			m.desc,
			labels,
			nil,
		)

		labelValues := []string{s.Label, s.UUID}
		if len(m.extraLabelValue) > 0 {
			labelValues = append(labelValues, m.extraLabelValue...)
		}

		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			m.value,
			labelValues...,
		)
	}
}

func (c *btrfsCollector) getAllocationStats(a string, s *btrfs.AllocationStats) []btrfsMetric {
	metrics := []btrfsMetric{
		{
			name:            "reserved_bytes",
			desc:            "Amount of space reserved for a data type",
			value:           float64(s.ReservedBytes),
			extraLabel:      []string{"type"},
			extraLabelValue: []string{a},
		},
	}

	metrics = append(metrics, c.getLayoutStats(a, "single", s.Single)...)
	metrics = append(metrics, c.getLayoutStats(a, "dup", s.Dup)...)
	metrics = append(metrics, c.getLayoutStats(a, "raid0", s.Raid0)...)
	metrics = append(metrics, c.getLayoutStats(a, "raid1", s.Raid1)...)
	metrics = append(metrics, c.getLayoutStats(a, "raid5", s.Raid5)...)
	metrics = append(metrics, c.getLayoutStats(a, "raid6", s.Raid6)...)
	metrics = append(metrics, c.getLayoutStats(a, "raid10", s.Raid10)...)

	return metrics
}

func (c *btrfsCollector) getLayoutStats(a, l string, s *btrfs.LayoutUsage) []btrfsMetric {
	if s == nil {
		return nil
	}

	return []btrfsMetric{
		{
			name:            "used_bytes",
			desc:            "Amount of used space by a layout/data type",
			value:           float64(s.UsedBytes),
			extraLabel:      []string{"type", "mode"},
			extraLabelValue: []string{a, l},
		},
		{
			name:            "total_bytes",
			desc:            "Amount of space allocated for a layout/data type",
			value:           float64(s.TotalBytes),
			extraLabel:      []string{"type", "mode"},
			extraLabelValue: []string{a, l},
		},
		{
			name:            "ratio",
			desc:            "Data allocation ratio for a layout/data type",
			value:           s.Ratio,
			extraLabel:      []string{"type", "mode"},
			extraLabelValue: []string{a, l},
		},
	}
}
