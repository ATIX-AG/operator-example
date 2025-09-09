package controller

import (
	"context"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ATIX-AG/operator-example/api/v1alpha1"
)

// DemoReconciler reconciles a Demo object
type DemoReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=demo.atix.de.atix.de,resources=demoes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=demo.atix.de.atix.de,resources=demoes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=demo.atix.de.atix.de,resources=demoes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Demo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *DemoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// get object
	var obj v1alpha1.Demo
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// we maintain two patches: one for status, one for metadata (e.g., finalizer changes)
	statusPatch := client.MergeFrom(obj.DeepCopy())

	// if custom resource is marked for deletion, start deleting process
	if !obj.DeletionTimestamp.IsZero() {
		r.setPhase(&obj, v1alpha1.DemoPhaseTerminating)
		r.setCondition(&obj, "Ready", metav1.ConditionFalse, "WaitingForTermination", "Waiting for terminating")
		r.setCondition(&obj, "Progressing", metav1.ConditionTrue, "Termination", "Terminating in progress")
	}

	switch obj.Status.Phase {
	case "":
		// set phase to 'pending' to start the process
		r.setPhase(&obj, v1alpha1.DemoPhasePending)
		r.setCondition(&obj, "Ready", metav1.ConditionFalse, "WaitingForDeployment", "Waiting for deployment")
		r.setCondition(&obj, "Progressing", metav1.ConditionTrue, "Deployment", "Deployment in progress")

	case v1alpha1.DemoPhasePending:
		// create all resources and set phase to 'running'
		if err := r.ensureDeployment(ctx, &obj); err != nil {
			return r.failOrRequeue(ctx, &obj, statusPatch, err)
		}
		if err := r.ensureService(ctx, &obj); err != nil {
			return r.failOrRequeue(ctx, &obj, statusPatch, err)
		}

		r.setPhase(&obj, v1alpha1.DemoPhaseRunning)
		r.setCondition(&obj, "Ready", metav1.ConditionFalse, "WaitingForStartup", "Waiting for startup")
		r.setCondition(&obj, "Progressing", metav1.ConditionTrue, "Startup", "Startup in progress")

	case v1alpha1.DemoPhaseRunning:
		// waiting for app to startup (health-checks)
		ready, err := r.healthCheck(ctx, &obj)
		if err != nil {
			return r.failOrRequeue(ctx, &obj, statusPatch, err)
		}
		if ready {
			r.setCondition(&obj, "Ready", metav1.ConditionTrue, "Ready", "Ready")
		} else {
			r.setCondition(&obj, "Ready", metav1.ConditionFalse, "NotReady", "Not ready")
		}
	}

	// patch status only once per reconciliation
	if err := r.Status().Patch(ctx, &obj, statusPatch); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DemoReconciler) healthCheck(ctx context.Context, obj *v1alpha1.Demo) (bool, error) {
	var podList coreV1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(obj.Namespace)); err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == coreV1.PodReady && cond.Status == coreV1.ConditionTrue {
				ready = true
				break
			}
		}

		if ready {
			return true, nil
		}
	}

	return false, nil
}

func (r *DemoReconciler) ensureDeployment(ctx context.Context, obj *v1alpha1.Demo) error {
	dep := &appsV1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		desired := generateDeployment(obj)

		if dep.Labels == nil {
			dep.Labels = map[string]string{}
		}
		for k, v := range desired.Labels {
			dep.Labels[k] = v
		}

		return controllerutil.SetControllerReference(obj, dep, r.Scheme)
	})

	return err
}

func (r *DemoReconciler) ensureService(ctx context.Context, obj *v1alpha1.Demo) error {
	dep := &coreV1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		desired := generateService(obj)

		if dep.Labels == nil {
			dep.Labels = map[string]string{}
		}
		for k, v := range desired.Labels {
			dep.Labels[k] = v
		}

		return controllerutil.SetControllerReference(obj, dep, r.Scheme)
	})

	return err
}

// setPhase updates the phase of the given ManagedCluster if the current phase differs from the specified new phase.
func (r *DemoReconciler) setPhase(demo *v1alpha1.Demo, phase v1alpha1.DemoPhase) {
	if demo.Status.Phase != phase {
		demo.Status.Phase = phase
	}
}

// setCondition updates the status of a ManagedCluster with the specified condition type, status, reason, and message.
func (r *DemoReconciler) setCondition(demo *v1alpha1.Demo, conditionType string, status metav1.ConditionStatus, reason, msg string) {
	apimeta.SetStatusCondition(&demo.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *DemoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Demo{}).
		Named("demo").
		Complete(r)
}

func generateDeployment(demo *v1alpha1.Demo) *appsV1.Deployment {
	return &appsV1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deploy",
			Namespace: demo.Namespace,
			Labels:    make(map[string]string),
		},
		Spec: appsV1.DeploymentSpec{
			Replicas: ptr.To(int32(2)),
			Selector: &metav1.LabelSelector{MatchLabels: make(map[string]string)},
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: make(map[string]string)},
				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.27-alpine",
							Ports: []coreV1.ContainerPort{{ContainerPort: 80, Name: "http"}},
							ReadinessProbe: &coreV1.Probe{
								ProbeHandler: coreV1.ProbeHandler{
									HTTPGet: &coreV1.HTTPGetAction{Path: "/", Port: intstr.FromString("http")},
								},
								InitialDelaySeconds: 3,
								PeriodSeconds:       5,
							},
							LivenessProbe: &coreV1.Probe{
								ProbeHandler: coreV1.ProbeHandler{
									HTTPGet: &coreV1.HTTPGetAction{Path: "/", Port: intstr.FromString("http")},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}
}

func generateService(demo *v1alpha1.Demo) *coreV1.Service {
	return &coreV1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-svc",
			Namespace: demo.Namespace,
			Labels:    make(map[string]string),
		},
		Spec: coreV1.ServiceSpec{
			Selector: make(map[string]string),
			Type:     coreV1.ServiceTypeNodePort,
			Ports: []coreV1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					NodePort:   30080,
					Protocol:   coreV1.ProtocolTCP,
				},
			},
		},
	}
}

func (r *DemoReconciler) failOrRequeue(ctx context.Context, o *v1alpha1.Demo, statusPatch client.Patch, err error) (ctrl.Result, error) {
	r.setCondition(o, "Progressing", metav1.ConditionTrue, "Retrying", err.Error())
	_ = r.Status().Patch(ctx, o, statusPatch)
	return ctrl.Result{}, err
}
