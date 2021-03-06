package servicefabric

// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Code generated by Microsoft (R) AutoRest Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"context"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/tracing"
	"net/http"
)

// MeshServiceReplicaClient is the service Fabric REST Client APIs allows management of Service Fabric clusters,
// applications and services.
type MeshServiceReplicaClient struct {
	BaseClient
}

// NewMeshServiceReplicaClient creates an instance of the MeshServiceReplicaClient client.
func NewMeshServiceReplicaClient() MeshServiceReplicaClient {
	return NewMeshServiceReplicaClientWithBaseURI(DefaultBaseURI)
}

// NewMeshServiceReplicaClientWithBaseURI creates an instance of the MeshServiceReplicaClient client.
func NewMeshServiceReplicaClientWithBaseURI(baseURI string) MeshServiceReplicaClient {
	return MeshServiceReplicaClient{NewWithBaseURI(baseURI)}
}

// Get gets the information about the service replica with the given name. The information include the description and
// other properties of the service replica.
// Parameters:
// applicationResourceName - the identity of the application.
// serviceResourceName - the identity of the service.
// replicaName - service Fabric replica name.
func (client MeshServiceReplicaClient) Get(ctx context.Context, applicationResourceName string, serviceResourceName string, replicaName string) (result ServiceReplicaDescription, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/MeshServiceReplicaClient.Get")
		defer func() {
			sc := -1
			if result.Response.Response != nil {
				sc = result.Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	req, err := client.GetPreparer(ctx, applicationResourceName, serviceResourceName, replicaName)
	if err != nil {
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "Get", nil, "Failure preparing request")
		return
	}

	resp, err := client.GetSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "Get", resp, "Failure sending request")
		return
	}

	result, err = client.GetResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "Get", resp, "Failure responding to request")
	}

	return
}

// GetPreparer prepares the Get request.
func (client MeshServiceReplicaClient) GetPreparer(ctx context.Context, applicationResourceName string, serviceResourceName string, replicaName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"applicationResourceName": applicationResourceName,
		"replicaName":             replicaName,
		"serviceResourceName":     serviceResourceName,
	}

	const APIVersion = "6.4-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/Resources/Applications/{applicationResourceName}/Services/{serviceResourceName}/Replicas/{replicaName}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// GetSender sends the Get request. The method will close the
// http.Response Body if it receives an error.
func (client MeshServiceReplicaClient) GetSender(req *http.Request) (*http.Response, error) {
	sd := autorest.GetSendDecorators(req.Context(), autorest.DoRetryForStatusCodes(client.RetryAttempts, client.RetryDuration, autorest.StatusCodesForRetry...))
	return autorest.SendWithSender(client, req, sd...)
}

// GetResponder handles the response to the Get request. The method always
// closes the http.Response Body.
func (client MeshServiceReplicaClient) GetResponder(resp *http.Response) (result ServiceReplicaDescription, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// List gets the information about all replicas of a service. The information include the description and other
// properties of the service replica.
// Parameters:
// applicationResourceName - the identity of the application.
// serviceResourceName - the identity of the service.
func (client MeshServiceReplicaClient) List(ctx context.Context, applicationResourceName string, serviceResourceName string) (result PagedServiceReplicaDescriptionList, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/MeshServiceReplicaClient.List")
		defer func() {
			sc := -1
			if result.Response.Response != nil {
				sc = result.Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	req, err := client.ListPreparer(ctx, applicationResourceName, serviceResourceName)
	if err != nil {
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "List", nil, "Failure preparing request")
		return
	}

	resp, err := client.ListSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "List", resp, "Failure sending request")
		return
	}

	result, err = client.ListResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "servicefabric.MeshServiceReplicaClient", "List", resp, "Failure responding to request")
	}

	return
}

// ListPreparer prepares the List request.
func (client MeshServiceReplicaClient) ListPreparer(ctx context.Context, applicationResourceName string, serviceResourceName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"applicationResourceName": applicationResourceName,
		"serviceResourceName":     serviceResourceName,
	}

	const APIVersion = "6.4-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/Resources/Applications/{applicationResourceName}/Services/{serviceResourceName}/Replicas", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// ListSender sends the List request. The method will close the
// http.Response Body if it receives an error.
func (client MeshServiceReplicaClient) ListSender(req *http.Request) (*http.Response, error) {
	sd := autorest.GetSendDecorators(req.Context(), autorest.DoRetryForStatusCodes(client.RetryAttempts, client.RetryDuration, autorest.StatusCodesForRetry...))
	return autorest.SendWithSender(client, req, sd...)
}

// ListResponder handles the response to the List request. The method always
// closes the http.Response Body.
func (client MeshServiceReplicaClient) ListResponder(resp *http.Response) (result PagedServiceReplicaDescriptionList, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}
