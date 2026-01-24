/*
  Copyright 2022 The Fluid Authors.

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

package efc

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/tools/record"

	"github.com/fluid-cloudnative/fluid/pkg/common"
	"k8s.io/utils/ptr"

	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/ddc/base"
	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	ctrlhelper "github.com/fluid-cloudnative/fluid/pkg/ctrl"
)

const (
	healthCheckTestNamespace = "big-data"
)

var _ = Describe("EFCEngine Health Check", Label("pkg.ddc.efc.health_check_test.go"), func() {
	Describe("CheckRuntimeHealthy", func() {
		Context("when all components are healthy", func() {
			It("should return no error", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 0,
					},
				}
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data-worker-endpoints",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).NotTo(HaveOccurred())
			})
		})

		Context("when master is not healthy", func() {
			It("should return an error", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 0,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 0,
					},
				}
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data-worker-endpoints",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "master-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "master-no-health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("master-no-health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("master-no-health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).To(HaveOccurred())
			})
		})

		Context("when worker is not healthy", func() {
			It("should return an error", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](2),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      2,
						ReadyReplicas: 0,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 0,
					},
				}
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data-worker-endpoints",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "worker-no-health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("worker-no-health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("worker-no-health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).To(HaveOccurred())
			})
		})

		Context("when worker is partially healthy", func() {
			It("should return no error", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](2),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      2,
						ReadyReplicas: 1,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 0,
					},
				}
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data-worker-endpoints",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-partial-health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "worker-partial-health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("worker-partial-health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("worker-partial-health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).NotTo(HaveOccurred())
			})
		})

		Context("when fuse is not healthy", func() {
			It("should return no error (fluid assumes fuse is always healthy)", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 1,
					},
				}
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data-worker-endpoints",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fuse-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "fuse-no-health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("fuse-no-health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("fuse-no-health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).NotTo(HaveOccurred())
			})
		})

		Context("when endpoints config map is not found", func() {
			It("should return an error", func() {
				efcRuntime := &datav1alpha1.EFCRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "endpoints-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
					Spec: datav1alpha1.EFCRuntimeSpec{
						Replicas: 1,
						Fuse:     datav1alpha1.EFCFuseSpec{},
					},
				}
				master := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "endpoints-no-health-data-master",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				worker := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "endpoints-no-health-data-worker",
						Namespace: healthCheckTestNamespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: ptr.To[int32](1),
						Selector: &metav1.LabelSelector{},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:      1,
						ReadyReplicas: 1,
					},
				}
				fuse := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "endpoints-no-health-data-fuse",
						Namespace: healthCheckTestNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 0,
					},
				}
				// ConfigMap with wrong name (simulates not found)
				workerEndPoints := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "123",
						Namespace: healthCheckTestNamespace,
					},
					Data: map[string]string{
						WorkerEndpointsDataName: workerEndpointsConfigMapData,
					},
				}
				data := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "endpoints-no-health-data",
						Namespace: healthCheckTestNamespace,
					},
				}

				s := runtime.NewScheme()
				s.AddKnownTypes(datav1alpha1.GroupVersion, efcRuntime)
				s.AddKnownTypes(datav1alpha1.GroupVersion, data)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, worker)
				s.AddKnownTypes(appsv1.SchemeGroupVersion, fuse)
				s.AddKnownTypes(v1.SchemeGroupVersion, workerEndPoints)
				err := v1.AddToScheme(s)
				Expect(err).NotTo(HaveOccurred())

				mockClient := fake.NewFakeClientWithScheme(s, efcRuntime, data, worker, master, fuse, workerEndPoints)
				e := &EFCEngine{
					runtime:   efcRuntime,
					name:      "endpoints-no-health-data",
					namespace: healthCheckTestNamespace,
					Client:    mockClient,
					Log:       ctrl.Log.WithName("endpoints-no-health-data"),
					Recorder:  record.NewFakeRecorder(1),
				}

				runtimeInfo, err := base.BuildRuntimeInfo("endpoints-no-health-data", healthCheckTestNamespace, common.EFCRuntime)
				Expect(err).NotTo(HaveOccurred())

				e.Helper = ctrlhelper.BuildHelper(runtimeInfo, mockClient, e.Log)

				healthError := e.CheckRuntimeHealthy()
				Expect(healthError).To(HaveOccurred())
			})
		})
	})
})
