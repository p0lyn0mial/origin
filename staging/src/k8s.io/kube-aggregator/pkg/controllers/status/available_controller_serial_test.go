package apiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics/legacyregistry"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	listers "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestUnavailableGauge check if unavailableGauge is properly set
//
// Note:
// the order in which the test scenarios are executed matters because unavailableGauge is a global variable
func TestUnavailableGauge(t *testing.T) {
	tests := []struct {
		name string

		apiServiceName     string
		apiServices        []runtime.Object
		services           []*v1.Service
		endpoints          []*v1.Endpoints
		forceDiscoveryFail bool

		expectedAvailability          apiregistrationv1.APIServiceCondition
		expectUnavailableGaugeToBeSet bool
	}{
		{
			name:           "a service was on before and it is off now - unavailableGauge is set to 1",
			apiServiceName: "remote.group",
			apiServices: []runtime.Object{func() *apiregistrationv1.APIService {
				srv := NewRemoteAPIServiceForTest("remote.group")
				srv.Status.Conditions = append(srv.Status.Conditions, apiregistrationv1.APIServiceCondition{
					Type:               apiregistrationv1.Available,
					Status:             apiregistrationv1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
				})
				return srv
			}()},
			services:           []*v1.Service{NewServiceForTest("foo", "bar", TestServicePort, TestServicePortName)},
			endpoints:          []*v1.Endpoints{NewEndpointsWithAddressForTest("foo", "bar", TestServicePort, TestServicePortName)},
			forceDiscoveryFail: true,
			expectedAvailability: apiregistrationv1.APIServiceCondition{
				Type:    apiregistrationv1.Available,
				Status:  apiregistrationv1.ConditionFalse,
				Reason:  "FailedDiscoveryCheck",
				Message: `failing or missing response from`,
			},
			expectUnavailableGaugeToBeSet: true,
		},

		{
			name: "a service was off before and it is on now - unavailableGauge must be set to 0",
			//              the previous scenario set the value of the metric to 1
			//              meanwhile the service became available and this was observed by a different instance
			//              thus we must update the metric based on the current state
			apiServiceName: "remote.group",
			apiServices: []runtime.Object{func() *apiregistrationv1.APIService {
				srv := NewRemoteAPIServiceForTest("remote.group")
				srv.Status.Conditions = append(srv.Status.Conditions, apiregistrationv1.APIServiceCondition{
					Type:               apiregistrationv1.Available,
					Status:             apiregistrationv1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
				})
				return srv
			}()},
			services:  []*v1.Service{NewServiceForTest("foo", "bar", TestServicePort, TestServicePortName)},
			endpoints: []*v1.Endpoints{NewEndpointsWithAddressForTest("foo", "bar", TestServicePort, TestServicePortName)},
			expectedAvailability: apiregistrationv1.APIServiceCondition{
				Type:    apiregistrationv1.Available,
				Status:  apiregistrationv1.ConditionTrue,
				Reason:  "Passed",
				Message: `all checks passed`,
			},
		},

		{
			name:           "a service was off before and it is off now - unavailableGauge is set to 1",
			apiServiceName: "remote.group",
			apiServices: []runtime.Object{func() *apiregistrationv1.APIService {
				srv := NewRemoteAPIServiceForTest("remote.group")
				srv.Status.Conditions = append(srv.Status.Conditions, apiregistrationv1.APIServiceCondition{
					Type:               apiregistrationv1.Available,
					Status:             apiregistrationv1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
				})
				return srv
			}()},
			services:           []*v1.Service{NewServiceForTest("foo", "bar", TestServicePort, TestServicePortName)},
			endpoints:          []*v1.Endpoints{NewEndpointsWithAddressForTest("foo", "bar", TestServicePort, TestServicePortName)},
			forceDiscoveryFail: true,
			expectedAvailability: apiregistrationv1.APIServiceCondition{
				Type:    apiregistrationv1.Available,
				Status:  apiregistrationv1.ConditionFalse,
				Reason:  "FailedDiscoveryCheck",
				Message: `failing or missing response from`,
			},
			expectUnavailableGaugeToBeSet: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tc.apiServices...)
			apiServiceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			serviceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			endpointsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, obj := range tc.apiServices {
				apiServiceIndexer.Add(obj)
			}
			for _, obj := range tc.services {
				serviceIndexer.Add(obj)
			}
			for _, obj := range tc.endpoints {
				endpointsIndexer.Add(obj)
			}

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.forceDiscoveryFail {
					w.WriteHeader(http.StatusOK)
				}
				w.WriteHeader(http.StatusForbidden)
			}))
			defer testServer.Close()

			c := AvailableConditionController{
				apiServiceClient: fakeClient.ApiregistrationV1(),
				apiServiceLister: listers.NewAPIServiceLister(apiServiceIndexer),
				serviceLister:    v1listers.NewServiceLister(serviceIndexer),
				endpointsLister:  v1listers.NewEndpointsLister(endpointsIndexer),
				discoveryClient:  testServer.Client(),
				serviceResolver:  &FakeServiceResolver{Url: testServer.URL},
			}

			c.sync(tc.apiServiceName)

			// ought to have one action writing status
			if e, a := 1, len(fakeClient.Actions()); e != a {
				t.Fatalf("%v expected %v, got %v", tc.name, e, fakeClient.Actions())
			}

			action, ok := fakeClient.Actions()[0].(clienttesting.UpdateAction)
			if !ok {
				t.Fatalf("%v got %v", tc.name, ok)
			}

			if e, a := 1, len(action.GetObject().(*apiregistrationv1.APIService).Status.Conditions); e != a {
				t.Fatalf("%v expected %v, got %v", tc.name, e, action.GetObject())
			}
			condition := action.GetObject().(*apiregistrationv1.APIService).Status.Conditions[0]
			if e, a := tc.expectedAvailability.Type, condition.Type; e != a {
				t.Errorf("%v expected %v, got %#v", tc.name, e, condition)
			}
			if e, a := tc.expectedAvailability.Status, condition.Status; e != a {
				t.Errorf("%v expected %v, got %#v", tc.name, e, condition)
			}
			if e, a := tc.expectedAvailability.Reason, condition.Reason; e != a {
				t.Errorf("%v expected %v, got %#v", tc.name, e, condition)
			}
			if e, a := tc.expectedAvailability.Message, condition.Message; !strings.HasPrefix(a, e) {
				t.Errorf("%v expected %v, got %#v", tc.name, e, condition)
			}
			if condition.LastTransitionTime.IsZero() {
				t.Error("expected lastTransitionTime to be non-zero")
			}

			registry := legacyregistry.DefaultGatherer
			registryMetrics, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}

			gVal := 0.0
			for _, registryMetric := range registryMetrics {
				if registryMetric != nil && *registryMetric.Name == unavailableGauge.Name {
					unavailableMetric := registryMetric.Metric[0]
					gVal = unavailableMetric.Gauge.GetValue()
					break
				}

			}

			if gVal != 0.0 && tc.expectUnavailableGaugeToBeSet == false {
				t.Fatalf("expected unavailableGauge to NOT be set but it WAS %f", gVal)
			}
			if gVal == 0.0 && tc.expectUnavailableGaugeToBeSet {
				t.Fatal("expected unavailableGauge to be SET but it WASN'T")
			}
		})
	}
}
