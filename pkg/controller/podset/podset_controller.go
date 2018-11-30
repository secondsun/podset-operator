package podset

import (
	"context"
	"log"
	"math/rand"
	"strconv"
	"time"

	podsetv1alpha1 "github.com/secondsun/podset-operator/pkg/apis/podset/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PodSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePodSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("podset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PodSet
	err = c.Watch(&source.Kind{Type: &podsetv1alpha1.PodSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PodSet
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &podsetv1alpha1.PodSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePodSet{}

// ReconcilePodSet reconciles a PodSet object
type ReconcilePodSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PodSet object and makes changes based on the state read
// and what is in the PodSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePodSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Jazzy Jeff - Reconciling PodSet %s/%s\n", request.Namespace, request.Name)

	// Fetch the PodSet instance
	instance := &podsetv1alpha1.PodSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	podCount := instance.Spec.Replicas
	podNames := instance.Status.PodNames
	var removeNames []string
	var keepNames []string
	for _, podName := range podNames {
		pod := newPodForCR(instance)
		pod.Name = podName
		// Check if this Pod already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Printf("Pod %s has been removed. Updating removeNames\n", pod.Name)
			removeNames = append(removeNames, podName)
		} else if err == nil {
			log.Printf("Pod %s exists. Updating keepNames\n", pod.Name)
			keepNames = append(keepNames, podName)
		}
	}

	if len(keepNames) == podCount {
		log.Printf("Pods all found\n")
		return reconcile.Result{}, nil
	}

	for createCount := podCount - len(keepNames); createCount > 0; createCount-- {
		// Define a new Pod object
		pod := newPodForCR(instance)

		// Set PodSet instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		log.Printf("Creating a new Pod %s/%s\n", pod.Namespace, pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err == nil {
			log.Printf("Created pod, appending pod name to podset/%s\n", pod.Name)
			keepNames = append(keepNames, pod.Name)
		} else {
			log.Printf("Error Creating/%s\n", pod.Name)
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue

	}

	instance.Status.PodNames = keepNames
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		log.Printf("Error Appending updating podset\n")
		return reconcile.Result{}, err
	}
	log.Printf("Updated podset/%s\n", instance.Name)
	// Pod already exists - don't requeue
	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *podsetv1alpha1.PodSet) *corev1.Pod {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod-" + strconv.FormatInt(r1.Int63(), 16),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}
