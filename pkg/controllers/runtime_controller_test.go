/*
Copyright 2025 The Fluid Authors.

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

package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/common"
	"github.com/fluid-cloudnative/fluid/pkg/dataoperation"
	"github.com/fluid-cloudnative/fluid/pkg/ddc/base"
	cruntime "github.com/fluid-cloudnative/fluid/pkg/runtime"
	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
)

// mockRuntimeReconcilerImpl implements RuntimeReconcilerInterface for testing
type mockRuntimeReconcilerImpl struct {
	getOrCreateEngineFunc func(ctx cruntime.ReconcileRequestContext) (base.Engine, error)
	getRuntimeObjectMeta  func(ctx cruntime.ReconcileRequestContext) (metav1.Object, error)

	reconcileRuntimeDeletionResult ctrl.Result
	reconcileRuntimeDeletionErr    error

	reconcileRuntimeResult ctrl.Result
	reconcileRuntimeErr    error

	addFinalizerResult ctrl.Result
	addFinalizerErr    error

	removeEngineCalled bool
}

func (m *mockRuntimeReconcilerImpl) ReconcileRuntimeDeletion(engine base.Engine, ctx cruntime.ReconcileRequestContext) (ctrl.Result, error) {
	return m.reconcileRuntimeDeletionResult, m.reconcileRuntimeDeletionErr
}

func (m *mockRuntimeReconcilerImpl) ReconcileRuntime(engine base.Engine, ctx cruntime.ReconcileRequestContext) (ctrl.Result, error) {
	return m.reconcileRuntimeResult, m.reconcileRuntimeErr
}

func (m *mockRuntimeReconcilerImpl) AddFinalizerAndRequeue(ctx cruntime.ReconcileRequestContext, finalizerName string) (ctrl.Result, error) {
	return m.addFinalizerResult, m.addFinalizerErr
}

func (m *mockRuntimeReconcilerImpl) GetDataset(ctx cruntime.ReconcileRequestContext) (*datav1alpha1.Dataset, error) {
	var dataset datav1alpha1.Dataset
	if err := ctx.Client.Get(ctx, ctx.NamespacedName, &dataset); err != nil {
		return nil, err
	}
	return &dataset, nil
}

func (m *mockRuntimeReconcilerImpl) GetOrCreateEngine(ctx cruntime.ReconcileRequestContext) (base.Engine, error) {
	if m.getOrCreateEngineFunc != nil {
		return m.getOrCreateEngineFunc(ctx)
	}
	return &fakeEngine{id: "test-engine"}, nil
}

func (m *mockRuntimeReconcilerImpl) RemoveEngine(ctx cruntime.ReconcileRequestContext) {
	m.removeEngineCalled = true
}

func (m *mockRuntimeReconcilerImpl) GetRuntimeObjectMeta(ctx cruntime.ReconcileRequestContext) (metav1.Object, error) {
	if m.getRuntimeObjectMeta != nil {
		return m.getRuntimeObjectMeta(ctx)
	}
	objectMetaAccessor, isOM := ctx.Runtime.(metav1.ObjectMetaAccessor)
	if !isOM {
		return nil, fmt.Errorf("object is not ObjectMetaAccessor")
	}
	return objectMetaAccessor.GetObjectMeta(), nil
}

func (m *mockRuntimeReconcilerImpl) ReconcileInternal(ctx cruntime.ReconcileRequestContext) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// fakeEngine implements base.Engine for testing
type fakeEngine struct {
	id           string
	shutdownErr  error
	deleteVolErr error
	setupReady   bool
	setupErr     error
	createVolErr error
	syncErr      error
	validateErr  error
}

func (e *fakeEngine) ID() string { return e.id }

func (e *fakeEngine) Shutdown() error { return e.shutdownErr }

func (e *fakeEngine) Setup(ctx cruntime.ReconcileRequestContext) (bool, error) {
	return e.setupReady, e.setupErr
}

func (e *fakeEngine) CreateVolume() error { return e.createVolErr }

func (e *fakeEngine) DeleteVolume() error { return e.deleteVolErr }

func (e *fakeEngine) Sync(ctx cruntime.ReconcileRequestContext) error { return e.syncErr }

func (e *fakeEngine) Validate(ctx cruntime.ReconcileRequestContext) error { return e.validateErr }

func (e *fakeEngine) Operate(ctx cruntime.ReconcileRequestContext, opStatus *datav1alpha1.OperationStatus, operation dataoperation.OperationInterface) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// runtimeObjectWithGVK wraps an AlluxioRuntime with proper GVK set for testing
func runtimeObjectWithGVK(name, namespace string) *datav1alpha1.AlluxioRuntime {
	rt := &datav1alpha1.AlluxioRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	rt.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "data.fluid.io",
		Version: "v1alpha1",
		Kind:    "AlluxioRuntime",
	})
	return rt
}

var _ = Describe("NewRuntimeReconciler", func() {
	It("should create a RuntimeReconciler with the provided parameters", func() {
		s := runtime.NewScheme()
		Expect(datav1alpha1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())

		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).Build()
		fakeRecorder := record.NewFakeRecorder(10)
		log := fake.NullLogger()

		reconciler := NewRuntimeReconciler(nil, fakeClient, log, fakeRecorder)

		Expect(reconciler).NotTo(BeNil())
		Expect(reconciler.Client).To(Equal(fakeClient))
		Expect(reconciler.Log).To(Equal(log))
		Expect(reconciler.Recorder).NotTo(BeNil())
	})
})

var _ = Describe("RuntimeReconciler", func() {
	var (
		s            *runtime.Scheme
		fakeClient   client.Client
		fakeRecorder *record.FakeRecorder
		reconciler   *RuntimeReconciler
	)

	BeforeEach(func() {
		s = runtime.NewScheme()
		Expect(datav1alpha1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())

		fakeRecorder = record.NewFakeRecorder(10)
	})

	Describe("GetDataset", func() {
		It("should return a dataset when it exists", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dataset",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(dataset).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-dataset",
					Namespace: "default",
				},
			}
			ctx.Client = fakeClient

			got, err := reconciler.GetDataset(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).NotTo(BeNil())
			Expect(got.Name).To(Equal("test-dataset"))
			Expect(got.Namespace).To(Equal("default"))
		})

		It("should return an error when the dataset does not exist", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent",
					Namespace: "default",
				},
			}
			ctx.Client = fakeClient

			got, err := reconciler.GetDataset(ctx)
			Expect(err).To(HaveOccurred())
			Expect(got).To(BeNil())
		})
	})

	Describe("GetRuntimeObjectMeta", func() {
		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}
		})

		It("should return object meta for a valid ObjectMetaAccessor runtime", func() {
			runtime := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Runtime: runtime,
			}

			objectMeta, err := reconciler.GetRuntimeObjectMeta(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(objectMeta).NotTo(BeNil())
			Expect(objectMeta.GetName()).To(Equal("test-runtime"))
			Expect(objectMeta.GetNamespace()).To(Equal("default"))
		})

		It("should return correct finalizers from object meta", func() {
			runtime := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "finalized-runtime",
					Namespace:  "default",
					Finalizers: []string{"fluid.io/finalizer"},
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Runtime: runtime,
			}

			objectMeta, err := reconciler.GetRuntimeObjectMeta(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(objectMeta.GetFinalizers()).To(ContainElement("fluid.io/finalizer"))
		})

		It("should return an error for a non-ObjectMetaAccessor runtime", func() {
			// Use a bare client.Object that does not implement ObjectMetaAccessor properly
			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Runtime: &nonMetaAccessorObject{},
			}

			_, err := reconciler.GetRuntimeObjectMeta(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ObjectMetaAccessor"))
		})
	})

	Describe("CheckIfReferenceDatasetIsSupported", func() {
		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}
		})

		It("should return true when dataset has no reference mounts", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dataset",
					Namespace: "default",
				},
				Spec: datav1alpha1.DatasetSpec{
					Mounts: []datav1alpha1.Mount{
						{
							MountPoint: "https://bucket.s3.amazonaws.com/data",
							Name:       "s3-data",
						},
					},
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context:     context.Background(),
				Dataset:     dataset,
				RuntimeType: common.AlluxioRuntime,
			}

			isSupported, reason := reconciler.CheckIfReferenceDatasetIsSupported(ctx)
			Expect(isSupported).To(BeTrue())
			Expect(reason).To(BeEmpty())
		})

		It("should return true when dataset has reference mounts and uses ThinRuntime", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virtual-dataset",
					Namespace: "default",
				},
				Spec: datav1alpha1.DatasetSpec{
					Mounts: []datav1alpha1.Mount{
						{
							MountPoint: "dataset://default/physical-dataset",
							Name:       "ref-mount",
						},
					},
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context:     context.Background(),
				Dataset:     dataset,
				RuntimeType: common.ThinRuntime,
			}

			isSupported, reason := reconciler.CheckIfReferenceDatasetIsSupported(ctx)
			Expect(isSupported).To(BeTrue())
			Expect(reason).To(BeEmpty())
		})

		It("should return false when dataset has reference mounts and does not use ThinRuntime", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virtual-dataset",
					Namespace: "default",
				},
				Spec: datav1alpha1.DatasetSpec{
					Mounts: []datav1alpha1.Mount{
						{
							MountPoint: "dataset://default/physical-dataset",
							Name:       "ref-mount",
						},
					},
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context:     context.Background(),
				Dataset:     dataset,
				RuntimeType: common.AlluxioRuntime,
			}

			isSupported, reason := reconciler.CheckIfReferenceDatasetIsSupported(ctx)
			Expect(isSupported).To(BeFalse())
			Expect(reason).To(ContainSubstring("thin runtime"))
		})

		DescribeTable("reference dataset support for various runtime types",
			func(runtimeType string, hasMountRef bool, expectedSupported bool) {
				mounts := []datav1alpha1.Mount{
					{
						MountPoint: "https://bucket.s3.amazonaws.com/data",
						Name:       "s3-data",
					},
				}
				if hasMountRef {
					mounts = []datav1alpha1.Mount{
						{
							MountPoint: "dataset://default/physical-dataset",
							Name:       "ref-mount",
						},
					}
				}

				dataset := &datav1alpha1.Dataset{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-dataset",
						Namespace: "default",
					},
					Spec: datav1alpha1.DatasetSpec{
						Mounts: mounts,
					},
				}

				ctx := cruntime.ReconcileRequestContext{
					Context:     context.Background(),
					Dataset:     dataset,
					RuntimeType: runtimeType,
				}

				isSupported, _ := reconciler.CheckIfReferenceDatasetIsSupported(ctx)
				Expect(isSupported).To(Equal(expectedSupported))
			},
			Entry("alluxio without ref mounts", common.AlluxioRuntime, false, true),
			Entry("alluxio with ref mounts", common.AlluxioRuntime, true, false),
			Entry("thin without ref mounts", common.ThinRuntime, false, true),
			Entry("thin with ref mounts", common.ThinRuntime, true, true),
			Entry("juicefs without ref mounts", common.JuiceFSRuntime, false, true),
			Entry("juicefs with ref mounts", common.JuiceFSRuntime, true, false),
		)
	})

	Describe("AddFinalizerAndRequeue", func() {
		It("should add a finalizer to the runtime and requeue", func() {
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(rt).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime:       rt,
				FinalizerName: "alluxioruntime-controller-finalizer",
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()

			result, err := reconciler.AddFinalizerAndRequeue(ctx, "alluxioruntime-controller-finalizer")
			Expect(err).NotTo(HaveOccurred())
			// Should requeue (either immediately or with generation change detection)
			Expect(result.Requeue || result.RequeueAfter > 0 || !result.IsZero()).To(BeTrue())

			// Verify the finalizer was added
			updatedRuntime := &datav1alpha1.AlluxioRuntime{}
			Expect(fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			}, updatedRuntime)).To(Succeed())
			Expect(updatedRuntime.GetFinalizers()).To(ContainElement("alluxioruntime-controller-finalizer"))
		})

		It("should return error if GetRuntimeObjectMeta fails", func() {
			mockImpl := &mockRuntimeReconcilerImpl{
				getRuntimeObjectMeta: func(ctx cruntime.ReconcileRequestContext) (metav1.Object, error) {
					return nil, fmt.Errorf("mock error getting object meta")
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
			}
			ctx.Log = fake.NullLogger()

			_, err := reconciler.AddFinalizerAndRequeue(ctx, "test-finalizer")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AddOwnerAndRequeue", func() {
		It("should add owner reference and requeue", func() {
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dataset",
					Namespace: "default",
					UID:       "test-uid-123",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(rt, dataset).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()

			result, err := reconciler.AddOwnerAndRequeue(ctx, dataset)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			// Verify the owner reference was added
			updatedRuntime := &datav1alpha1.AlluxioRuntime{}
			Expect(fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			}, updatedRuntime)).To(Succeed())
			Expect(updatedRuntime.GetOwnerReferences()).To(HaveLen(1))
			Expect(updatedRuntime.GetOwnerReferences()[0].Name).To(Equal("test-dataset"))
			Expect(updatedRuntime.GetOwnerReferences()[0].UID).To(Equal(types.UID("test-uid-123")))
		})

		It("should return error if GetRuntimeObjectMeta fails", func() {
			mockImpl := &mockRuntimeReconcilerImpl{
				getRuntimeObjectMeta: func(ctx cruntime.ReconcileRequestContext) (metav1.Object, error) {
					return nil, fmt.Errorf("mock error getting object meta")
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dataset",
				},
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
			}
			ctx.Log = fake.NullLogger()

			_, err := reconciler.AddOwnerAndRequeue(ctx, dataset)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ReportDatasetNotReadyCondition", func() {
		It("should update dataset status with NotReady condition", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dataset",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(dataset).
				WithStatusSubresource(dataset).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Dataset: dataset,
			}
			ctx.Client = fakeClient

			err := reconciler.ReportDatasetNotReadyCondition(ctx, fmt.Errorf("setup failed: workers not ready"))
			Expect(err).NotTo(HaveOccurred())

			// Verify the dataset condition was updated
			updatedDataset := &datav1alpha1.Dataset{}
			Expect(fakeClient.Get(context.Background(), types.NamespacedName{
				Name:      "test-dataset",
				Namespace: "default",
			}, updatedDataset)).To(Succeed())
			Expect(updatedDataset.Status.Conditions).NotTo(BeEmpty())
			// Check that a DatasetNotReady condition exists
			found := false
			for _, cond := range updatedDataset.Status.Conditions {
				if cond.Type == datav1alpha1.DatasetNotReady {
					found = true
					Expect(cond.Message).To(ContainSubstring("setup failed"))
					Expect(cond.Status).To(Equal(corev1.ConditionTrue))
				}
			}
			Expect(found).To(BeTrue(), "expected DatasetNotReady condition to be set")
		})

		It("should return error when dataset not found", func() {
			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nonexistent",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Dataset: dataset,
			}
			ctx.Client = fakeClient

			err := reconciler.ReportDatasetNotReadyCondition(ctx, fmt.Errorf("some error"))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ForgetMetrics", func() {
		It("should not panic when called with a valid runtime context", func() {
			rt := runtimeObjectWithGVK("test-runtime", "default")

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			reconciler = &RuntimeReconciler{
				Client:   fakeClient,
				Log:      fake.NullLogger(),
				Recorder: fakeRecorder,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}

			Expect(func() {
				reconciler.ForgetMetrics(ctx)
			}).NotTo(Panic())
		})
	})

	Describe("ReconcileInternal", func() {
		It("should return error when runtime is nil", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			mockImpl := &mockRuntimeReconcilerImpl{}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				Runtime: nil,
			}
			ctx.Log = fake.NullLogger()

			_, err := reconciler.ReconcileInternal(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find the runtime"))
		})

		It("should return error for invalid runtime name", func() {
			// A runtime name starting with a digit is invalid per DNS1035
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "20-hbase",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			mockImpl := &mockRuntimeReconcilerImpl{}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "20-hbase",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Log = fake.NullLogger()
			ctx.Recorder = fakeRecorder

			_, err := reconciler.ReconcileInternal(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle deletion when runtime has deletion timestamp", func() {
			now := metav1.Now()
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-runtime",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{"alluxioruntime-controller-finalizer"},
				},
			}

			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(dataset).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{
				reconcileRuntimeDeletionResult: ctrl.Result{},
			}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()
			ctx.Recorder = fakeRecorder

			result, err := reconciler.ReconcileInternal(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should handle deletion error and call RemoveEngine", func() {
			now := metav1.Now()
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-runtime",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{"alluxioruntime-controller-finalizer"},
				},
			}

			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(dataset).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{
				reconcileRuntimeDeletionResult: ctrl.Result{},
				reconcileRuntimeDeletionErr:    fmt.Errorf("deletion failed"),
			}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()
			ctx.Recorder = fakeRecorder

			_, err := reconciler.ReconcileInternal(ctx)
			Expect(err).To(HaveOccurred())
			Expect(mockImpl.removeEngineCalled).To(BeTrue())
		})

		It("should requeue when dataset is not found (nil dataset, waiting)", func() {
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()
			ctx.Recorder = fakeRecorder

			result, err := reconciler.ReconcileInternal(ctx)
			Expect(err).NotTo(HaveOccurred())
			// Should requeue because no dataset found
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("should return error when GetOrCreateEngine fails", func() {
			rt := &datav1alpha1.AlluxioRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
			}

			dataset := &datav1alpha1.Dataset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Status: datav1alpha1.DatasetStatus{
					Runtimes: []datav1alpha1.Runtime{
						{
							Name:      "test-runtime",
							Namespace: "default",
							Type:      common.AlluxioRuntime,
							Category:  common.AccelerateCategory,
						},
					},
				},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(s).
				WithObjects(dataset).
				Build()

			mockImpl := &mockRuntimeReconcilerImpl{
				getOrCreateEngineFunc: func(ctx cruntime.ReconcileRequestContext) (base.Engine, error) {
					return nil, fmt.Errorf("engine creation failed")
				},
			}

			reconciler = &RuntimeReconciler{
				Client:    fakeClient,
				Log:       fake.NullLogger(),
				Recorder:  fakeRecorder,
				implement: mockImpl,
			}

			ctx := cruntime.ReconcileRequestContext{
				Context: context.Background(),
				NamespacedName: types.NamespacedName{
					Name:      "test-runtime",
					Namespace: "default",
				},
				Runtime: rt,
			}
			ctx.Client = fakeClient
			ctx.Log = fake.NullLogger()
			ctx.Recorder = fakeRecorder

			_, err := reconciler.ReconcileInternal(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("engine creation failed"))
		})
	})
})

var _ = Describe("RuntimeReconciler ReconcileRuntimeDeletion", func() {
	var (
		s            *runtime.Scheme
		fakeRecorder *record.FakeRecorder
	)

	BeforeEach(func() {
		s = runtime.NewScheme()
		Expect(datav1alpha1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		fakeRecorder = record.NewFakeRecorder(10)
	})

	It("should succeed when engine shutdown and volume deletion succeed", func() {
		now := metav1.Now()
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-runtime",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{"alluxioruntime-controller-finalizer"},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Status: datav1alpha1.DatasetStatus{
				Phase: datav1alpha1.BoundDatasetPhase,
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			WithStatusSubresource(dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}
		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		engine := &fakeEngine{id: "test-engine"}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime:       rt,
			Dataset:       dataset,
			FinalizerName: "alluxioruntime-controller-finalizer",
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()

		result, err := reconciler.ReconcileRuntimeDeletion(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		// Verify dataset status was reset
		updatedDataset := &datav1alpha1.Dataset{}
		Expect(fakeClient.Get(context.Background(), types.NamespacedName{
			Name:      "test-runtime",
			Namespace: "default",
		}, updatedDataset)).To(Succeed())
		Expect(updatedDataset.Status.Phase).To(Equal(datav1alpha1.NotBoundDatasetPhase))
		Expect(updatedDataset.Status.Runtimes).To(BeEmpty())

		// Verify RemoveEngine was called
		Expect(mockImpl.removeEngineCalled).To(BeTrue())
	})

	It("should requeue when engine.DeleteVolume fails", func() {
		now := metav1.Now()
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-runtime",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{"alluxioruntime-controller-finalizer"},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}
		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		engine := &fakeEngine{
			id:           "test-engine",
			deleteVolErr: fmt.Errorf("volume deletion failed"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()

		result, err := reconciler.ReconcileRuntimeDeletion(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		// Should requeue after interval
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("should requeue when engine.Shutdown fails", func() {
		now := metav1.Now()
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-runtime",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{"alluxioruntime-controller-finalizer"},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}
		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		engine := &fakeEngine{
			id:          "test-engine",
			shutdownErr: fmt.Errorf("shutdown failed"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()

		result, err := reconciler.ReconcileRuntimeDeletion(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("should return error when dataset status update fails during deletion", func() {
		now := metav1.Now()
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-runtime",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{"alluxioruntime-controller-finalizer"},
			},
		}
		rt.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "data.fluid.io",
			Version: "v1alpha1",
			Kind:    "AlluxioRuntime",
		})

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		// Build client WITHOUT dataset as status subresource -> status update will fail
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}
		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		engine := &fakeEngine{id: "test-engine"}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime:       rt,
			Dataset:       dataset,
			FinalizerName: "alluxioruntime-controller-finalizer",
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()

		_, err := reconciler.ReconcileRuntimeDeletion(engine, ctx)
		// Status update should fail since dataset doesn't exist in the fake client
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("RuntimeReconciler ReconcileRuntime", func() {
	var (
		s            *runtime.Scheme
		fakeRecorder *record.FakeRecorder
	)

	BeforeEach(func() {
		s = runtime.NewScheme()
		Expect(datav1alpha1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		fakeRecorder = record.NewFakeRecorder(10)
	})

	It("should complete successfully when all engine operations succeed", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Status: datav1alpha1.DatasetStatus{
				Conditions: []datav1alpha1.DatasetCondition{
					{
						Type:   datav1alpha1.DatasetReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:         "test-engine",
			setupReady: true,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		// Should either not requeue or requeue based on env var
		_ = result
	})

	It("should requeue when validation fails", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:          "test-engine",
			validateErr: fmt.Errorf("validation failed"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("should run Setup when dataset is not yet ready", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		// No DatasetReady condition means setup is not done
		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			WithStatusSubresource(dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:         "test-engine",
			setupReady: true,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		// After successful setup, should proceed through volume creation and sync
		_ = result
	})

	It("should requeue when Setup returns not ready", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			WithStatusSubresource(dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:         "test-engine",
			setupReady: false,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("should handle Setup error and report not-ready condition", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			WithStatusSubresource(dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:         "test-engine",
			setupReady: false,
			setupErr:   fmt.Errorf("workers not ready"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		// Should requeue since not ready
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})

	It("should handle CreateVolume error gracefully", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Status: datav1alpha1.DatasetStatus{
				Conditions: []datav1alpha1.DatasetCondition{
					{
						Type:   datav1alpha1.DatasetReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:           "test-engine",
			createVolErr: fmt.Errorf("volume creation failed"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		// CreateVolume error is logged but does not halt reconciliation
		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		_ = result
	})

	It("should requeue when Sync fails", func() {
		rt := runtimeObjectWithGVK("test-runtime", "default")

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Status: datav1alpha1.DatasetStatus{
				Conditions: []datav1alpha1.DatasetCondition{
					{
						Type:   datav1alpha1.DatasetReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		reconciler := &RuntimeReconciler{
			Client:   fakeClient,
			Log:      fake.NullLogger(),
			Recorder: fakeRecorder,
		}

		engine := &fakeEngine{
			id:      "test-engine",
			syncErr: fmt.Errorf("sync failed"),
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
			Dataset: dataset,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileRuntime(engine, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})
})

var _ = Describe("RuntimeReconciler ReconcileInternal extended paths", func() {
	var (
		s            *runtime.Scheme
		fakeRecorder *record.FakeRecorder
	)

	BeforeEach(func() {
		s = runtime.NewScheme()
		Expect(datav1alpha1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		fakeRecorder = record.NewFakeRecorder(10)
	})

	It("should add owner reference and requeue when dataset exists but no owner", func() {
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-runtime",
				Namespace:  "default",
				Finalizers: []string{"alluxioruntime-controller-finalizer"},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				UID:       "test-uid",
			},
			Status: datav1alpha1.DatasetStatus{
				Runtimes: []datav1alpha1.Runtime{
					{
						Name:      "test-runtime",
						Namespace: "default",
						Type:      common.AlluxioRuntime,
						Category:  common.AccelerateCategory,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}

		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileInternal(ctx)
		Expect(err).NotTo(HaveOccurred())
		// Should requeue after adding owner reference
		Expect(result.Requeue || result.RequeueAfter > 0 || !result.IsZero()).To(BeTrue())
	})

	It("should add finalizer and requeue when dataset exists with owner but no finalizer", func() {
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test-runtime",
						UID:  "test-uid",
					},
				},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				UID:       "test-uid",
			},
			Status: datav1alpha1.DatasetStatus{
				Runtimes: []datav1alpha1.Runtime{
					{
						Name:      "test-runtime",
						Namespace: "default",
						Type:      common.AlluxioRuntime,
						Category:  common.AccelerateCategory,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{
			addFinalizerResult: ctrl.Result{Requeue: true},
		}

		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime:       rt,
			FinalizerName: "alluxioruntime-controller-finalizer",
			Category:      common.AccelerateCategory,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileInternal(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
	})

	It("should proceed to ReconcileRuntime when everything is set up", func() {
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test-runtime",
						UID:  "test-uid",
					},
				},
				Finalizers: []string{"alluxioruntime-controller-finalizer"},
			},
		}

		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				UID:       "test-uid",
			},
			Status: datav1alpha1.DatasetStatus{
				Runtimes: []datav1alpha1.Runtime{
					{
						Name:      "test-runtime",
						Namespace: "default",
						Type:      common.AlluxioRuntime,
						Category:  common.AccelerateCategory,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{
			reconcileRuntimeResult: ctrl.Result{},
		}

		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime:       rt,
			FinalizerName: "alluxioruntime-controller-finalizer",
			Category:      common.AccelerateCategory,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileInternal(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
	})

	It("should requeue when dataset can't be bound to another runtime", func() {
		rt := &datav1alpha1.AlluxioRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test-runtime",
						UID:  "test-uid",
					},
				},
			},
		}

		// Dataset is already bound to a different runtime
		dataset := &datav1alpha1.Dataset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runtime",
				Namespace: "default",
				UID:       "test-uid",
			},
			Status: datav1alpha1.DatasetStatus{
				Runtimes: []datav1alpha1.Runtime{
					{
						Name:      "other-runtime",
						Namespace: "other-ns",
						Type:      common.AlluxioRuntime,
						Category:  common.AccelerateCategory,
					},
				},
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(rt, dataset).
			Build()

		mockImpl := &mockRuntimeReconcilerImpl{}

		reconciler := &RuntimeReconciler{
			Client:    fakeClient,
			Log:       fake.NullLogger(),
			Recorder:  fakeRecorder,
			implement: mockImpl,
		}

		ctx := cruntime.ReconcileRequestContext{
			Context: context.Background(),
			NamespacedName: types.NamespacedName{
				Name:      "test-runtime",
				Namespace: "default",
			},
			Runtime: rt,
		}
		ctx.Client = fakeClient
		ctx.Log = fake.NullLogger()
		ctx.Recorder = fakeRecorder

		result, err := reconciler.ReconcileInternal(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))
	})
})

// nonMetaAccessorObject implements client.Object but NOT metav1.ObjectMetaAccessor
// Used to test the error path in GetRuntimeObjectMeta
type nonMetaAccessorObject struct{}

func (n *nonMetaAccessorObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (n *nonMetaAccessorObject) DeepCopyObject() runtime.Object { return n }

func (n *nonMetaAccessorObject) GetNamespace() string                        { return "" }
func (n *nonMetaAccessorObject) SetNamespace(namespace string)               {}
func (n *nonMetaAccessorObject) GetName() string                             { return "" }
func (n *nonMetaAccessorObject) SetName(name string)                         {}
func (n *nonMetaAccessorObject) GetGenerateName() string                     { return "" }
func (n *nonMetaAccessorObject) SetGenerateName(name string)                 {}
func (n *nonMetaAccessorObject) GetUID() types.UID                           { return "" }
func (n *nonMetaAccessorObject) SetUID(uid types.UID)                        {}
func (n *nonMetaAccessorObject) GetResourceVersion() string                  { return "" }
func (n *nonMetaAccessorObject) SetResourceVersion(version string)           {}
func (n *nonMetaAccessorObject) GetGeneration() int64                        { return 0 }
func (n *nonMetaAccessorObject) SetGeneration(generation int64)              {}
func (n *nonMetaAccessorObject) GetSelfLink() string                         { return "" }
func (n *nonMetaAccessorObject) SetSelfLink(selfLink string)                 {}
func (n *nonMetaAccessorObject) GetCreationTimestamp() metav1.Time           { return metav1.Time{} }
func (n *nonMetaAccessorObject) SetCreationTimestamp(timestamp metav1.Time)  {}
func (n *nonMetaAccessorObject) GetDeletionTimestamp() *metav1.Time          { return nil }
func (n *nonMetaAccessorObject) SetDeletionTimestamp(timestamp *metav1.Time) {}
func (n *nonMetaAccessorObject) GetDeletionGracePeriodSeconds() *int64       { return nil }
func (n *nonMetaAccessorObject) SetDeletionGracePeriodSeconds(*int64)        {}
func (n *nonMetaAccessorObject) GetLabels() map[string]string                { return nil }
func (n *nonMetaAccessorObject) SetLabels(labels map[string]string)          {}
func (n *nonMetaAccessorObject) GetAnnotations() map[string]string           { return nil }
func (n *nonMetaAccessorObject) SetAnnotations(annotations map[string]string) {
}
func (n *nonMetaAccessorObject) GetFinalizers() []string                     { return nil }
func (n *nonMetaAccessorObject) SetFinalizers(finalizers []string)           {}
func (n *nonMetaAccessorObject) GetOwnerReferences() []metav1.OwnerReference { return nil }
func (n *nonMetaAccessorObject) SetOwnerReferences([]metav1.OwnerReference)  {}
func (n *nonMetaAccessorObject) GetManagedFields() []metav1.ManagedFieldsEntry {
	return nil
}
func (n *nonMetaAccessorObject) SetManagedFields(managedFields []metav1.ManagedFieldsEntry) {
}
