package apiserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	listers "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"
)

const (
	TestServicePort     = 1234
	TestServicePortName = "testPort"
)

func NewAvailableConditionControllerForTest(
	apiServiceClient apiregistrationclient.APIServicesGetter,
	apiServiceLister listers.APIServiceLister,
	serviceLister v1listers.ServiceLister,
	endpointsLister v1listers.EndpointsLister,
	discoveryClient *http.Client,
	serviceResolver ServiceResolver) *AvailableConditionController {

	c := &AvailableConditionController{
		apiServiceClient: apiServiceClient,
		apiServiceLister: apiServiceLister,
		serviceLister:    serviceLister,
		endpointsLister:  endpointsLister,
		discoveryClient:  discoveryClient,
		serviceResolver:  serviceResolver,

		queue: workqueue.NewNamedRateLimitingQueue(
			// We want a fairly tight requeue time.  The controller listens to the API, but because it relies on the routability of the
			// service network, it is possible for an external, non-watchable factor to affect availability.  This keeps
			// the maximum disruption time to a minimum, but it does prevent hot loops.
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 30*time.Second),
			"AvailableConditionController"),
	}

	return c
}

func (c *AvailableConditionController) SyncForTest(key string) error {
	return c.sync(key)
}

func (c *AvailableConditionController) AddServiceForTest(obj interface{}) {
	c.addService(obj)
}

func (c *AvailableConditionController) AddAPIServiceForTest(obj interface{}) {
	c.addAPIService(obj)
}

func (c *AvailableConditionController) UpdateServiceForTest(obj, obj2 interface{}) {
	c.updateService(obj, obj2)
}

func (c *AvailableConditionController) DeleteServiceForTest(obj interface{}) {
	c.deleteAPIService(obj)
}

func UpdateAPIServiceStatusForTest(client apiregistrationclient.APIServicesGetter, originalAPIService, newAPIService *apiregistrationv1.APIService) (*apiregistrationv1.APIService, error) {
	return updateAPIServiceStatus(client, originalAPIService, newAPIService)
}

func NewEndpointsForTest(namespace, name string) *v1.Endpoints {
	return &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
}

func NewEndpointsWithAddressForTest(namespace, name string, port int32, portName string) *v1.Endpoints {
	return &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: "val",
					},
				},
				Ports: []v1.EndpointPort{
					{
						Name: portName,
						Port: port,
					},
				},
			},
		},
	}
}

func NewServiceForTest(namespace, name string, port int32, portName string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				{Port: port, Name: portName},
			},
		},
	}
}

func NewLocalAPIServiceForTest(name string) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func NewRemoteAPIServiceForTest(name string) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:   strings.SplitN(name, ".", 2)[0],
			Version: strings.SplitN(name, ".", 2)[1],
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "foo",
				Name:      "bar",
				Port:      pointer.Int32Ptr(TestServicePort),
			},
		},
	}
}

func SetupAPIServicesForTest(apiServices []*apiregistrationv1.APIService) (*AvailableConditionController, *fake.Clientset) {
	fakeClient := fake.NewSimpleClientset()
	apiServiceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	serviceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	endpointsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	for _, o := range apiServices {
		apiServiceIndexer.Add(o)
	}

	c := NewAvailableConditionControllerForTest(
		fakeClient.ApiregistrationV1(),
		listers.NewAPIServiceLister(apiServiceIndexer),
		v1listers.NewServiceLister(serviceIndexer),
		v1listers.NewEndpointsLister(endpointsIndexer),
		testServer.Client(),
		&FakeServiceResolver{Url: testServer.URL},
	)
	for _, svc := range apiServices {
		c.AddAPIServiceForTest(svc)
	}
	return c, fakeClient
}

type FakeServiceResolver struct {
	Url string
}

func (f *FakeServiceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	return url.Parse(f.Url)
}
