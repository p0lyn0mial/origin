/*
Copyright 2017 The Kubernetes Authors.

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

package apiserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	apiregistration "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	listers "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"
	apiserver "k8s.io/kube-aggregator/pkg/controllers/status"
)

func BenchmarkBuildCache(b *testing.B) {
	apiServiceName := "remote.group"
	// model 1 APIService pointing at a given service, and 30 pointing at local group/versions
	apiServices := []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest(apiServiceName)}
	for i := 0; i < 30; i++ {
		apiServices = append(apiServices, apiserver.NewLocalAPIServiceForTest(fmt.Sprintf("local.group%d", i)))
	}
	// model one service backing an API service, and 100 unrelated services
	services := []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)}
	for i := 0; i < 100; i++ {
		services = append(services, apiserver.NewServiceForTest("foo", fmt.Sprintf("bar%d", i), apiserver.TestServicePort, apiserver.TestServicePortName))
	}
	c, _ := apiserver.SetupAPIServicesForTest(apiServices)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 1; n <= b.N; n++ {
		for _, svc := range services {
			c.AddServiceForTest(svc)
		}
		for _, svc := range services {
			c.UpdateServiceForTest(svc, svc)
		}
		for _, svc := range services {
			c.DeleteServiceForTest(svc)
		}
	}
}

func TestBuildCache(t *testing.T) {
	tests := []struct {
		name string

		apiServiceName string
		apiServices    []*apiregistration.APIService
		services       []*v1.Service
		endpoints      []*v1.Endpoints

		expectedAvailability apiregistration.APIServiceCondition
	}{
		{
			name:           "api service",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, fakeClient := apiserver.SetupAPIServicesForTest(tc.apiServices)
			for _, svc := range tc.services {
				c.AddServiceForTest(svc)
			}

			c.SyncForTest(tc.apiServiceName)

			// ought to have one action writing status
			if e, a := 1, len(fakeClient.Actions()); e != a {
				t.Fatalf("%v expected %v, got %v", tc.name, e, fakeClient.Actions())
			}
		})
	}
}
func TestSync(t *testing.T) {
	tests := []struct {
		name string

		apiServiceName     string
		apiServices        []*apiregistration.APIService
		services           []*v1.Service
		endpoints          []*v1.Endpoints
		forceDiscoveryFail bool

		expectedAvailability apiregistration.APIServiceCondition
	}{
		{
			name:           "local",
			apiServiceName: "local.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewLocalAPIServiceForTest("local.group")},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionTrue,
				Reason:  "Local",
				Message: "Local APIServices are always available",
			},
		},
		{
			name:           "no service",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "not-bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "ServiceNotFound",
				Message: `service/bar in "foo" is not present`,
			},
		},
		{
			name:           "service on bad port",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services: []*v1.Service{{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "bar"},
				Spec: v1.ServiceSpec{
					Type: v1.ServiceTypeClusterIP,
					Ports: []v1.ServicePort{
						{Port: 6443},
					},
				},
			}},
			endpoints: []*v1.Endpoints{apiserver.NewEndpointsWithAddressForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "ServicePortError",
				Message: fmt.Sprintf(`service/bar in "foo" is not listening on port %d`, apiserver.TestServicePort),
			},
		},
		{
			name:           "no endpoints",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "EndpointsNotFound",
				Message: `cannot find endpoints for service/bar in "foo"`,
			},
		},
		{
			name:           "missing endpoints",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			endpoints:      []*v1.Endpoints{apiserver.NewEndpointsForTest("foo", "bar")},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "MissingEndpoints",
				Message: `endpoints for service/bar in "foo" have no addresses with port name "testPort"`,
			},
		},
		{
			name:           "wrong endpoint port name",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			endpoints:      []*v1.Endpoints{apiserver.NewEndpointsWithAddressForTest("foo", "bar", apiserver.TestServicePort, "wrongName")},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "MissingEndpoints",
				Message: fmt.Sprintf(`endpoints for service/bar in "foo" have no addresses with port name "%s"`, apiserver.TestServicePortName),
			},
		},
		{
			name:           "remote",
			apiServiceName: "remote.group",
			apiServices:    []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:       []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			endpoints:      []*v1.Endpoints{apiserver.NewEndpointsWithAddressForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionTrue,
				Reason:  "Passed",
				Message: `all checks passed`,
			},
		},
		{
			name:               "remote-bad-return",
			apiServiceName:     "remote.group",
			apiServices:        []*apiregistration.APIService{apiserver.NewRemoteAPIServiceForTest("remote.group")},
			services:           []*v1.Service{apiserver.NewServiceForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			endpoints:          []*v1.Endpoints{apiserver.NewEndpointsWithAddressForTest("foo", "bar", apiserver.TestServicePort, apiserver.TestServicePortName)},
			forceDiscoveryFail: true,
			expectedAvailability: apiregistration.APIServiceCondition{
				Type:    apiregistration.Available,
				Status:  apiregistration.ConditionFalse,
				Reason:  "FailedDiscoveryCheck",
				Message: `failing or missing response from`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
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

			c := apiserver.NewAvailableConditionControllerForTest(
				fakeClient.ApiregistrationV1(),
				listers.NewAPIServiceLister(apiServiceIndexer),
				v1listers.NewServiceLister(serviceIndexer),
				v1listers.NewEndpointsLister(endpointsIndexer),
				testServer.Client(),
				&apiserver.FakeServiceResolver{Url: testServer.URL},
			)
			c.SyncForTest(tc.apiServiceName)

			// ought to have one action writing status
			if e, a := 1, len(fakeClient.Actions()); e != a {
				t.Fatalf("%v expected %v, got %v", tc.name, e, fakeClient.Actions())
			}

			action, ok := fakeClient.Actions()[0].(clienttesting.UpdateAction)
			if !ok {
				t.Fatalf("%v got %v", tc.name, ok)
			}

			if e, a := 1, len(action.GetObject().(*apiregistration.APIService).Status.Conditions); e != a {
				t.Fatalf("%v expected %v, got %v", tc.name, e, action.GetObject())
			}
			condition := action.GetObject().(*apiregistration.APIService).Status.Conditions[0]
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
		})
	}
}

func TestUpdateAPIServiceStatus(t *testing.T) {
	foo := &apiregistration.APIService{Status: apiregistration.APIServiceStatus{Conditions: []apiregistration.APIServiceCondition{{Type: "foo"}}}}
	bar := &apiregistration.APIService{Status: apiregistration.APIServiceStatus{Conditions: []apiregistration.APIServiceCondition{{Type: "bar"}}}}

	fakeClient := fake.NewSimpleClientset()
	apiserver.UpdateAPIServiceStatusForTest(fakeClient.ApiregistrationV1().(apiregistrationclient.APIServicesGetter), foo, foo)
	if e, a := 0, len(fakeClient.Actions()); e != a {
		t.Error(spew.Sdump(fakeClient.Actions()))
	}

	fakeClient.ClearActions()
	apiserver.UpdateAPIServiceStatusForTest(fakeClient.ApiregistrationV1().(apiregistrationclient.APIServicesGetter), foo, bar)
	if e, a := 1, len(fakeClient.Actions()); e != a {
		t.Error(spew.Sdump(fakeClient.Actions()))
	}

}
