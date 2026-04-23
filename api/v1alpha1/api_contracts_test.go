/*
Copyright 2026 The Fluid Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("api v1alpha1 contracts", func() {
	Describe("MetadataSyncPolicy", func() {
		It("defaults auto sync to enabled when unset", func() {
			policy := &MetadataSyncPolicy{}

			Expect(policy.AutoSyncEnabled()).To(BeTrue())
		})

		It("reflects explicit auto sync values", func() {
			enabled := true
			disabled := false

			Expect((&MetadataSyncPolicy{AutoSync: &enabled}).AutoSyncEnabled()).To(BeTrue())
			Expect((&MetadataSyncPolicy{AutoSync: &disabled}).AutoSyncEnabled()).To(BeFalse())
		})
	})

	Describe("runtime helper methods", func() {
		It("returns worker replica values for runtime kinds that expose the helper", func() {
			Expect((&AlluxioRuntime{Spec: AlluxioRuntimeSpec{Replicas: 2}}).Replicas()).To(Equal(int32(2)))
			Expect((&GooseFSRuntime{Spec: GooseFSRuntimeSpec{Replicas: 3}}).Replicas()).To(Equal(int32(3)))
			Expect((&JuiceFSRuntime{Spec: JuiceFSRuntimeSpec{Replicas: 4}}).Replicas()).To(Equal(int32(4)))
			Expect((&JindoRuntime{Spec: JindoRuntimeSpec{Replicas: 5}}).Replicas()).To(Equal(int32(5)))
			Expect((&ThinRuntime{Spec: ThinRuntimeSpec{Replicas: 6}}).Replicas()).To(Equal(int32(6)))
			Expect((&VineyardRuntime{Spec: VineyardRuntimeSpec{Replicas: 7}}).Replicas()).To(Equal(int32(7)))
		})

		It("returns the address of the underlying status structs", func() {
			alluxio := &AlluxioRuntime{Status: RuntimeStatus{Selector: "alluxio"}}
			cache := &CacheRuntime{Status: CacheRuntimeStatus{Selector: "cache"}}
			goose := &GooseFSRuntime{Status: RuntimeStatus{Selector: "goose"}}
			jindo := &JindoRuntime{Status: RuntimeStatus{Selector: "jindo"}}
			juice := &JuiceFSRuntime{Status: RuntimeStatus{Selector: "juice"}}
			thin := &ThinRuntime{Status: RuntimeStatus{Selector: "thin"}}
			vineyard := &VineyardRuntime{Status: RuntimeStatus{Selector: "vineyard"}}

			Expect(alluxio.GetStatus()).To(BeIdenticalTo(&alluxio.Status))
			Expect(cache.GetStatus()).To(BeIdenticalTo(&cache.Status))
			Expect(goose.GetStatus()).To(BeIdenticalTo(&goose.Status))
			Expect(jindo.GetStatus()).To(BeIdenticalTo(&jindo.Status))
			Expect(juice.GetStatus()).To(BeIdenticalTo(&juice.Status))
			Expect(thin.GetStatus()).To(BeIdenticalTo(&thin.Status))
			Expect(vineyard.GetStatus()).To(BeIdenticalTo(&vineyard.Status))
		})
	})

	Describe("scheme registration", func() {
		It("registers the package runtime kinds into the unit test scheme", func() {
			cases := []struct {
				object runtime.Object
				kind   string
			}{
				{object: &AlluxioRuntime{}, kind: "AlluxioRuntime"},
				{object: &CacheRuntime{}, kind: "CacheRuntime"},
				{object: &DataBackup{}, kind: "DataBackup"},
				{object: &DataLoad{}, kind: "DataLoad"},
				{object: &DataMigrate{}, kind: "DataMigrate"},
				{object: &DataProcess{}, kind: "DataProcess"},
				{object: &Dataset{}, kind: "Dataset"},
				{object: &GooseFSRuntime{}, kind: "GooseFSRuntime"},
				{object: &JindoRuntime{}, kind: "JindoRuntime"},
				{object: &JuiceFSRuntime{}, kind: "JuiceFSRuntime"},
				{object: &ThinRuntime{}, kind: "ThinRuntime"},
				{object: &VineyardRuntime{}, kind: "VineyardRuntime"},
			}

			for _, tc := range cases {
				kinds, _, err := UnitTestScheme.ObjectKinds(tc.object)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds).To(ContainElement(GroupVersion.WithKind(tc.kind)))
			}
		})
	})
})
