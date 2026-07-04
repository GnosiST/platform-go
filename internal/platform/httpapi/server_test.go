package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
)

type adminResourceListTestPayload struct {
	Data struct {
		Resource string                    `json:"resource"`
		Items    []adminResourceRecordTest `json:"items"`
	} `json:"data"`
}

type adminResourceRecordTestPayload struct {
	Data struct {
		Record adminResourceRecordTest `json:"record"`
	} `json:"data"`
}

type adminResourceRecordTest struct {
	ID          string            `json:"id"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Values      map[string]string `json:"values"`
}

func TestHealthEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("GET /api/health body = %s", recorder.Body.String())
	}
}

func TestCapabilitiesEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}, {ID: "identity"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/capabilities status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"tenant"`) || !strings.Contains(body, `"id":"identity"`) {
		t.Fatalf("GET /api/capabilities body = %s", body)
	}
}

func TestAdminResourceListEndpointReturnsSeedRows(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/resources/tenants status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminResourceListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode resource list: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Resource != "tenants" {
		t.Fatalf("resource = %q, want tenants", payload.Data.Resource)
	}
	if len(payload.Data.Items) == 0 {
		t.Fatalf("expected seed rows, got none")
	}
	if payload.Data.Items[0].ID == "" || payload.Data.Items[0].Name == "" {
		t.Fatalf("seed row missing stable id or name: %+v", payload.Data.Items[0])
	}
}

func TestAdminResourceCreateUpdateDelete(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "dictionary"}}})
	createBody := bytes.NewBufferString(`{"code":"demo-status","name":"Demo Status","status":"enabled","description":"Demo status dictionary","values":{"scope":"global"}}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/dictionary-parameters", createBody)
	createRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("POST resource status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var createdPayload adminResourceRecordTestPayload
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createdPayload); err != nil {
		t.Fatalf("decode created resource: %v body = %s", err, createRecorder.Body.String())
	}
	created := createdPayload.Data.Record
	if created.ID == "" || created.Code != "demo-status" || created.Values["scope"] != "global" {
		t.Fatalf("created record mismatch: %+v", created)
	}

	updateBody := bytes.NewBufferString(`{"name":"Demo Status Updated","status":"disabled","description":"Updated","values":{"scope":"tenant"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/dictionary-parameters/"+created.ID, updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT resource status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updatedPayload adminResourceRecordTestPayload
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updatedPayload); err != nil {
		t.Fatalf("decode updated resource: %v body = %s", err, updateRecorder.Body.String())
	}
	updated := updatedPayload.Data.Record
	if updated.Name != "Demo Status Updated" || updated.Status != "disabled" || updated.Values["scope"] != "tenant" {
		t.Fatalf("updated record mismatch: %+v", updated)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/dictionary-parameters/"+created.ID, nil)

	server.Router().ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE resource status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/dictionary-parameters", nil)
	server.Router().ServeHTTP(listRecorder, listRequest)
	if strings.Contains(listRecorder.Body.String(), created.ID) {
		t.Fatalf("deleted record still present: %s", listRecorder.Body.String())
	}
}
