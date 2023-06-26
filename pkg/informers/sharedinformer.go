package informers

import (
	"context"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var _ SharedInformerFactory = &messageSharedInformerFactory{}

type messageSharedInformerFactory struct {
	defaultResync time.Duration
	namespace     string

	lock      sync.Mutex
	informers map[schema.GroupVersionResource]informers.GenericInformer
	// startedInformers is used for tracking which informers have been started.
	// This allows Start() to be called multiple times safely.
	startedInformers map[schema.GroupVersionResource]bool
	tweakListOptions TweakListOptionsFunc
	// wg tracks how many goroutines were started.
	wg sync.WaitGroup
	// shuttingDown is true when Shutdown has been called. It may still be running
	// because it needs to wait for goroutines.
	shuttingDown bool

	// normally we use the client to list/watch resources
	ctx          context.Context
	client       MQTT.Client
	signalTopic  string
	payloadTopic string
}

// NewSharedMessageInformerFactory constructs a new instance of metadataSharedInformerFactory for all namespaces.
func NewSharedMessageInformerFactory(ctx context.Context, client MQTT.Client, defaultResync time.Duration, signal, payload string) SharedInformerFactory {
	return NewFilteredSharedInformerFactory(ctx, client, defaultResync, metav1.NamespaceAll, nil, signal, payload)
}

// NewFilteredSharedInformerFactory constructs a new instance of metadataSharedInformerFactory.
// Listers obtained via this factory will be subject to the same filters as specified here.
func NewFilteredSharedInformerFactory(ctx context.Context, client MQTT.Client, defaultResync time.Duration, namespace string, tweakListOptions TweakListOptionsFunc, signal, payload string) SharedInformerFactory {
	return &messageSharedInformerFactory{
		ctx:              ctx,
		defaultResync:    defaultResync,
		namespace:        namespace,
		informers:        map[schema.GroupVersionResource]informers.GenericInformer{},
		startedInformers: make(map[schema.GroupVersionResource]bool),
		tweakListOptions: tweakListOptions,
		client:           client,
		signalTopic:      signal,
		payloadTopic:     payload,
	}
}

func (f *messageSharedInformerFactory) ForResource(gvr schema.GroupVersionResource) informers.GenericInformer {
	f.lock.Lock()
	defer f.lock.Unlock()
	key := gvr
	informer, exists := f.informers[key]
	if exists {
		return informer
	}

	informer = NewFilteredMetadataInformer(f.ctx, f.client, gvr, f.namespace, f.defaultResync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions, f.signalTopic, f.payloadTopic)
	f.informers[key] = informer

	return informer
}

// Start initializes all requested informers.
func (f *messageSharedInformerFactory) Start() {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Informer().Run(f.ctx.Done())
			f.startedInformers[informerType] = true
		}
	}
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (f *messageSharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	informers := func() map[schema.GroupVersionResource]cache.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		informers := map[schema.GroupVersionResource]cache.SharedIndexInformer{}
		for informerType, informer := range f.informers {
			if f.startedInformers[informerType] {
				informers[informerType] = informer.Informer()
			}
		}
		return informers
	}()

	res := map[schema.GroupVersionResource]bool{}
	for informType, informer := range informers {
		res[informType] = cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
	return res
}
