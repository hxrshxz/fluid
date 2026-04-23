/*
  Copyright 2023 The Fluid Authors.

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluid-cloudnative/fluid/pkg/common"
)

var _ = Describe("Dataset", func() {
	DescribeTable("RemoveDataOperationInProgress",
		func(status DatasetStatus, operationType string, name string, expected string, expectedRefs map[string]string) {
			dataset := &Dataset{Status: status}

			result := dataset.RemoveDataOperationInProgress(operationType, name)

			Expect(result).To(Equal(expected))
			Expect(dataset.Status.OperationRef).To(Equal(expectedRefs))
		},
		Entry("removes the only running operation",
			DatasetStatus{OperationRef: map[string]string{"DataLoad": "test1"}},
			"DataLoad",
			"test1",
			"",
			map[string]string{},
		),
		Entry("removes one operation from a comma-separated list",
			DatasetStatus{OperationRef: map[string]string{"DataLoad": "test1,test2"}},
			"DataLoad",
			"test1",
			"test2",
			map[string]string{"DataLoad": "test2"},
		),
		Entry("returns empty string when no operation map exists",
			DatasetStatus{},
			"DataLoad",
			"test1",
			"",
			nil,
		),
		Entry("does not create a blank entry for an unrelated operation type",
			DatasetStatus{OperationRef: map[string]string{"DataLoad": "test1"}},
			"DataBackup",
			"backup1",
			"",
			map[string]string{"DataLoad": "test1"},
		),
	)

	DescribeTable("SetDataOperationInProgress",
		func(status DatasetStatus, operationType string, name string, expected string) {
			dataset := &Dataset{Status: status}

			dataset.SetDataOperationInProgress(operationType, name)

			Expect(dataset.GetDataOperationInProgress(operationType)).To(Equal(expected))
		},
		Entry("initializes the operation map", DatasetStatus{}, "DataLoad", "test1", "test1"),
		Entry("appends a different operation name", DatasetStatus{OperationRef: map[string]string{"DataLoad": "test1"}}, "DataLoad", "test2", "test1,test2"),
		Entry("stores another operation type independently", DatasetStatus{OperationRef: map[string]string{"DataLoad": "test1"}}, "DataMigrate", "test", "test"),
		Entry("avoids duplicating the same operation name", DatasetStatus{OperationRef: map[string]string{"DataLoad": "test"}}, "DataLoad", "test", "test"),
	)

	It("returns false when a runtime does not match the requested binding target", func() {
		dataset := &Dataset{
			Status: DatasetStatus{
				Runtimes: []Runtime{{
					Name:      "runtime-a",
					Namespace: "fluid-system",
					Category:  common.AccelerateCategory,
				}},
			},
		}

		Expect(dataset.CanbeBound("runtime-b", "fluid-system", common.AccelerateCategory)).To(BeFalse())
	})

	It("returns true when a runtime matches the requested binding target", func() {
		dataset := &Dataset{
			Status: DatasetStatus{
				Runtimes: []Runtime{{
					Name:      "runtime-a",
					Namespace: "fluid-system",
					Category:  common.AccelerateCategory,
				}},
			},
		}

		Expect(dataset.CanbeBound("runtime-a", "fluid-system", common.AccelerateCategory)).To(BeTrue())
	})

	DescribeTable("IsExclusiveMode",
		func(mode PlacementMode, expected bool) {
			dataset := &Dataset{Spec: DatasetSpec{PlacementMode: mode}}

			Expect(dataset.IsExclusiveMode()).To(Equal(expected))
		},
		Entry("defaults to exclusive", DefaultMode, true),
		Entry("explicit exclusive mode", ExclusiveMode, true),
		Entry("shared mode", ShareMode, false),
	)

	It("preserves embedded metadata while mutating operation refs", func() {
		dataset := &Dataset{
			TypeMeta: metav1.TypeMeta{Kind: "Dataset"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dataset-sample",
				Namespace: "fluid-system",
			},
			Spec: DatasetSpec{
				Tolerations: []corev1.Toleration{{Key: "dedicated"}},
			},
			Status: DatasetStatus{},
		}

		dataset.SetDataOperationInProgress("DataLoad", "job-1")

		Expect(dataset.Name).To(Equal("dataset-sample"))
		Expect(dataset.Namespace).To(Equal("fluid-system"))
		Expect(dataset.Spec.Tolerations).To(HaveLen(1))
	})
})
