/*
Copyright 2021 The Fluid Authors.

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

package alluxio

import (
	"context"
	"fmt"

	"github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/ddc/base"
	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getTestAlluxioEngineNode creates and returns a test instance of AlluxioEngine.
func getTestAlluxioEngineNode(client client.Client, name string, namespace string, withRunTime bool) *AlluxioEngine {
	engine := &AlluxioEngine{
		runtime:     nil,
		name:        name,
		namespace:   namespace,
		Client:      client,
		runtimeInfo: nil,
		Log:         fake.NullLogger(),
	}
	if withRunTime {
		engine.runtime = &v1alpha1.AlluxioRuntime{}
		engine.runtimeInfo, _ = base.BuildRuntimeInfo(name, namespace, "alluxio")
	}
	return engine
}

var _ = Describe("SyncScheduleInfoToCacheNodes", func() {
	var (
		engine    *AlluxioEngine
		c         client.Client
		nodeNames []string
	)

	BeforeEach(func() {
		nodeNames = []string{}
	})

	Context("when a pod with controller reference is scheduled", func() {
		It("should label the node", func() {
			name := "spark"
			namespace := "big-data"
			worker := &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spark-worker",
					Namespace: namespace,
					UID:       "uid1",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":     "alluxio",
							"role":    "alluxio-worker",
							"release": name,
						},
					},
				},
			}
			pods := []*v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "spark-worker-0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{{
							Kind:       "StatefulSet",
							APIVersion: "apps/v1",
							Name:       "spark-worker",
							UID:        "uid1",
							Controller: ptr.To(true),
						}},
						Labels: map[string]string{
							"app":              "alluxio",
							"role":             "alluxio-worker",
							"release":          name,
							"fluid.io/dataset": "big-data-spark",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node1",
					},
				},
			}
			nodes := []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
			}

			runtimeObjs := []runtime.Object{worker}
			for _, pod := range pods {
				runtimeObjs = append(runtimeObjs, pod)
			}
			for _, node := range nodes {
				runtimeObjs = append(runtimeObjs, node)
			}

			c = fake.NewFakeClientWithScheme(testScheme, runtimeObjs...)
			engine = getTestAlluxioEngineNode(c, name, namespace, true)

			err := engine.SyncScheduleInfoToCacheNodes()
			Expect(err).NotTo(HaveOccurred())

			nodeList := &v1.NodeList{}
			datasetLabels, err := labels.Parse(fmt.Sprintf("%s=true", engine.runtimeInfo.GetCommonLabelName()))
			Expect(err).NotTo(HaveOccurred())

			err = c.List(context.TODO(), nodeList, &client.ListOptions{
				LabelSelector: datasetLabels,
			})
			Expect(err).NotTo(HaveOccurred())

			for _, node := range nodeList.Items {
				nodeNames = append(nodeNames, node.Name)
			}
			Expect(nodeNames).To(ConsistOf("node1"))
		})
	})

	Context("when a pod exists without label on node", func() {
		It("should add the label to the node", func() {
			name := "hbase"
			namespace := "big-data"
			worker := &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hbase-worker",
					Namespace: namespace,
					UID:       "uid2",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":     "alluxio",
							"role":    "alluxio-worker",
							"release": name,
						},
					},
				},
			}
			pods := []*v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hbase-worker-0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{{
							Kind:       "StatefulSet",
							APIVersion: "apps/v1",
							Name:       "hbase-worker",
							UID:        "uid2",
							Controller: ptr.To(true),
						}},
						Labels: map[string]string{
							"app":              "alluxio",
							"role":             "alluxio-worker",
							"release":          name,
							"fluid.io/dataset": "big-data-hbase",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node3",
					},
				},
			}
			nodes := []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
						Labels: map[string]string{
							"fluid.io/s-default-hbase": "true",
						},
					},
				},
			}

			runtimeObjs := []runtime.Object{worker}
			for _, pod := range pods {
				runtimeObjs = append(runtimeObjs, pod)
			}
			for _, node := range nodes {
				runtimeObjs = append(runtimeObjs, node)
			}

			c = fake.NewFakeClientWithScheme(testScheme, runtimeObjs...)
			engine = getTestAlluxioEngineNode(c, name, namespace, true)

			err := engine.SyncScheduleInfoToCacheNodes()
			Expect(err).NotTo(HaveOccurred())

			nodeList := &v1.NodeList{}
			datasetLabels, err := labels.Parse(fmt.Sprintf("%s=true", engine.runtimeInfo.GetCommonLabelName()))
			Expect(err).NotTo(HaveOccurred())

			err = c.List(context.TODO(), nodeList, &client.ListOptions{
				LabelSelector: datasetLabels,
			})
			Expect(err).NotTo(HaveOccurred())

			for _, node := range nodeList.Items {
				nodeNames = append(nodeNames, node.Name)
			}
			Expect(nodeNames).To(ConsistOf("node3"))
		})
	})

	Context("when a pod has no controller reference", func() {
		It("should not label the node", func() {
			name := "hbase-a"
			namespace := "big-data"
			worker := &appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hbase-a-worker",
					Namespace: namespace,
					UID:       "uid3",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":     "alluxio",
							"role":    "alluxio-worker",
							"release": name,
						},
					},
				},
			}
			pods := []*v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hbase-a-worker-0",
						Namespace: namespace,
						Labels: map[string]string{
							"app":              "alluxio",
							"role":             "alluxio-worker",
							"release":          name,
							"fluid.io/dataset": "big-data-hbase-a",
						},
					},
					Spec: v1.PodSpec{
						NodeName: "node5",
					},
				},
			}
			nodes := []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node5",
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "node4",
						Labels: map[string]string{
							"fluid.io/s-default-hbase-a": "true",
						},
					},
				},
			}

			runtimeObjs := []runtime.Object{worker}
			for _, pod := range pods {
				runtimeObjs = append(runtimeObjs, pod)
			}
			for _, node := range nodes {
				runtimeObjs = append(runtimeObjs, node)
			}

			c = fake.NewFakeClientWithScheme(testScheme, runtimeObjs...)
			engine = getTestAlluxioEngineNode(c, name, namespace, true)

			err := engine.SyncScheduleInfoToCacheNodes()
			Expect(err).NotTo(HaveOccurred())

			nodeList := &v1.NodeList{}
			datasetLabels, err := labels.Parse(fmt.Sprintf("%s=true", engine.runtimeInfo.GetCommonLabelName()))
			Expect(err).NotTo(HaveOccurred())

			err = c.List(context.TODO(), nodeList, &client.ListOptions{
				LabelSelector: datasetLabels,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(nodeList.Items).To(BeEmpty())
		})
	})
})
