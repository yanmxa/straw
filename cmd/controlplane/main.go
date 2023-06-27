package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/yanmxa/mqtt-informer/pkg/client"
	"github.com/yanmxa/mqtt-informer/pkg/config"
	"github.com/yanmxa/mqtt-informer/pkg/constant"
	"github.com/yanmxa/mqtt-informer/pkg/informers"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// ctx := context.Background()

	config := config.GetClientConfig()
	client := client.GetClient(config)

	informerFactory := informers.NewSharedMessageInformerFactory(ctx, client, 5*time.Minute,
		config.SignalTopic, config.PayloadTopic)

	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	informer := informerFactory.ForResource(gvr)

	restConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	if err != nil {
		panic(err.Error())
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

	// Create the Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			accessor, _ := meta.Accessor(obj)
			if _, ok := accessor.GetLabels()["mqtt-resource"]; !ok {
				return
			}
			accessor = convertToGlobalObj(accessor)
			validateNamespace(kubeClient, accessor.GetNamespace())

			accessor.SetResourceVersion("")
			accessor.SetManagedFields(nil)
			accessor.SetGeneration(0)
			unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(accessor)
			if err != nil {
				klog.Error(err)
				return
			}
			_, err = dynamicClient.Resource(gvr).Namespace(accessor.GetNamespace()).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
			if err != nil {
				klog.Error(err)
				return
			}

			klog.Infof("Added %s/%s", accessor.GetName(), accessor.GetNamespace())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldAccessor, _ := meta.Accessor(oldObj)
			newAccessor, _ := meta.Accessor(newObj)
			if _, ok := newAccessor.GetLabels()["mqtt-resource"]; !ok {
				return
			}
			newAccessor = convertToGlobalObj(newAccessor)

			oldUnstructuredObj, err := dynamicClient.Resource(gvr).Namespace(newAccessor.GetNamespace()).Get(ctx, newAccessor.GetName(), metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				return
			}

			newUnstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newAccessor)
			if err != nil {
				klog.Error(err)
				return
			}
			newUnstructuredObj["metadata"].(map[string]interface{})["resourceVersion"] = oldUnstructuredObj.GetResourceVersion()
			newUnstructuredObj["metadata"].(map[string]interface{})["uid"] = oldUnstructuredObj.GetUID()

			_, err = dynamicClient.Resource(gvr).Namespace(newAccessor.GetNamespace()).Update(ctx, &unstructured.Unstructured{Object: newUnstructuredObj}, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
			klog.Infof("Updated from %s/%s to %s/%s", oldAccessor.GetNamespace(), oldAccessor.GetName(), newAccessor.GetNamespace(), newAccessor.GetName())
		},
		DeleteFunc: func(obj interface{}) {
			accessor, _ := meta.Accessor(obj)
			if _, ok := accessor.GetLabels()["mqtt-resource"]; !ok {
				return
			}
			accessor = convertToGlobalObj(accessor)
			err := dynamicClient.Resource(gvr).Namespace(accessor.GetNamespace()).Delete(ctx, accessor.GetName(), metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
			klog.Infof("Deleted %s/%s", accessor.GetName(), accessor.GetNamespace())
		},
	})

	informerFactory.Start()
	<-ctx.Done()
}

func convertToGlobalObj(obj metav1.Object) metav1.Object {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	clusterName := obj.GetLabels()[constant.ClusterLabelKey]
	if namespace == "" {
		obj.SetName(clusterName + "." + name)
	} else {
		obj.SetName(namespace + "." + name)
		obj.SetNamespace(clusterName)
	}
	return obj
}

func validateNamespace(kubeClient *kubernetes.Clientset, namespace string) error {
	_, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})
	}

	return err
}
