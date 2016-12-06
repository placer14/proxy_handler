package proxyhandler

import (
	"bytes"
	"fmt"
	"github.com/jarcoal/httpmock"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func beforeTest() {
	httpmock.Activate()
	log.SetOutput(ioutil.Discard)
}

func afterTest() {
	log.SetOutput(os.Stderr)
	httpmock.DeactivateAndReset()

}

func TestResponseStatus(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedStatus := 999
	httpmock.RegisterResponder("GET", "http://hostname/", httpmock.NewBytesResponder(expectedStatus, nil))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://hostname/", nil)
	h, err := New("http://hostname")
	if err != nil {
		t.Fatalf("Failed creating handler: %v", err.Error())
	}
	h.ServeHTTP(recorder, req)

	if recorder.Code != expectedStatus {
		t.Fatalf("Expected status code not found\n\tExpected: %v\n\tActual: %v", expectedStatus, recorder.Code)
	}
}

func TestRequestBodyTransfer(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedBody := []byte("This is the expected body")
	httpmock.RegisterResponder("POST", "http://hostname/", func(r *http.Request) (*http.Response, error) {
		actualBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Request body unreadable: %v", err.Error())
		}

		if string(expectedBody) != string(actualBody) {
			t.Fatalf("Expected body not found\n\tExpected: %v\n\tActual: %v", expectedBody, actualBody)
		}
		return httpmock.NewStringResponse(200, ""), nil
	})

	req := httptest.NewRequest("POST", "http://hostname/", bytes.NewBuffer(expectedBody))

	h, _ := New("")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestResponseBodyTransfer(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedBody := []byte("This is the expected body")
	httpmock.RegisterResponder("GET", "http://hostname/", httpmock.NewBytesResponder(200, expectedBody))
	req := httptest.NewRequest("GET", "http://hostname/", nil)
	recorder := httptest.NewRecorder()

	h, _ := New("")
	h.ServeHTTP(recorder, req)
	actualBody, err := ioutil.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("Response body unreadable: %v", err.Error())
	}

	if string(expectedBody) != string(actualBody) {
		t.Fatalf("Expected body not found\n\tExpected: %v\n\tActual: %v", expectedBody, actualBody)
	}
}

func TestRequestHeaderTransfer(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedHeader := http.Header{
		"X-Foo": []string{"IMPORTANT"},
		"X-Bar": []string{"here; are_some; headers"},
	}
	httpmock.RegisterResponder("GET", "http://hostname/", func(r *http.Request) (*http.Response, error) {
		if !reflect.DeepEqual(r.Header, expectedHeader) {
			t.Fatalf("Unexpected headers\n\tExpected: %v\n\tActual: %v", expectedHeader, r.Header)
		}
		return httpmock.NewStringResponse(200, ""), nil
	})
	req := httptest.NewRequest("GET", "http://hostname/", nil)
	req.Header = expectedHeader

	h, _ := New("")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestResponseHeaderTransfer(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedHeader := http.Header{
		"Accept-Ranges":  []string{"bytes"},
		"Content-Length": []string{"6"},
		"Content-Type":   []string{"text/plain; charset=utf-8"},
		"Content-Range":  []string{"bytes 0-5/1862"},
	}
	mockResponder := httpmock.ResponderFromResponse(&http.Response{
		Header: expectedHeader,
		Body:   httpmock.NewRespBodyFromString(""),
	})
	httpmock.RegisterResponder("GET", "http://hostname/", mockResponder)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://hostname/", nil)

	h, _ := New("")
	h.ServeHTTP(recorder, req)

	if !reflect.DeepEqual(recorder.Header(), expectedHeader) {
		t.Fatalf("Unexpected headers\n\tExpected: %v\n\tActual: %v", expectedHeader, recorder.Header())
	}
}

func TestPostMethod(t *testing.T) {
	beforeTest()
	defer afterTest()

	expectedPostBody := `{"some":"json"}`
	success := false
	httpmock.RegisterResponder("POST", "http://hostname/", func(r *http.Request) (*http.Response, error) {
		success = true
		actualBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Response body unreadable: %v", err.Error())
		}
		if expectedPostBody != string(actualBody) {
			t.Fatalf("Body did not match\n\tExpected: %v\n\tActual: %v", expectedPostBody, string(actualBody))
		}
		return httpmock.NewStringResponse(200, ""), nil
	})
	req := httptest.NewRequest("POST", "http://hostname/", strings.NewReader(expectedPostBody))
	recorder := httptest.NewRecorder()

	h, _ := New("")
	h.ServeHTTP(recorder, req)
	if !success {
		t.Error("Expected POST responder to be executed")
	}
}

func TestProxyHandlesSpecificEndpoint(t *testing.T) {
	beforeTest()
	defer afterTest()

	success := false
	httpmock.RegisterResponder("GET", "http://google.com/foo", func(r *http.Request) (*http.Response, error) {
		success = true
		return httpmock.NewStringResponse(200, ""), nil
	})
	req := httptest.NewRequest("GET", "http://hostname/foo", nil)
	recorder := httptest.NewRecorder()

	h, _ := New("")
	h.HandleEndpoint("/foo", "//google.com/")
	h.ServeHTTP(recorder, req)

	if !success {
		t.Error("Expected handler to direct request to google.com host")
	}
}

func TestDefaultHostParsingFailure(t *testing.T) {
	expectedError := "proxy: invalid default host"
	_, err := New("http://192.168%31/") // invalid URL
	if err == nil {
		t.Fatal("Expected handler to return an error when default host cannot be parsed")
	}
	if !strings.HasPrefix(err.Error(), expectedError) {
		t.Errorf("Expected invalid default host error\nActual: %v\nExpected: %v", err.Error(), expectedError)
	}
}

func TestInvalidSchemeFails(t *testing.T) {
	expectedError := "proxy: invalid default host scheme"
	_, err := New("foobarbaz://localhost")
	if err == nil {
		t.Fatal("Expected handler to return an error when default scheme is invalid")
	}
	if !strings.HasPrefix(err.Error(), expectedError) {
		t.Errorf("Expected invalid scheme error\nActual: %v\nExpected: %v", err.Error(), expectedError)
	}
}

func TestDefaultHostIsSet(t *testing.T) {
	defaultHost := "http://192.168.1.1"
	h, _ := New(defaultHost) // invalid URL
	if h.defaultHostURL.String() != defaultHost {
		t.Fatal("Expected handler's DefaultHostURL to match argument")
	}
}

func TestDefaultHostIsUsedWhenMatchingRouteMissing(t *testing.T) {
	beforeTest()
	defer afterTest()
	success := false

	httpmock.RegisterResponder("GET", "http://notgoogle/", func(r *http.Request) (*http.Response, error) {
		success = true
		return httpmock.NewStringResponse(200, ""), nil
	})

	h, _ := New("http://notgoogle")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !success {
		t.Error("Expected default host to be requested")
	}
}

func TestHandleEndpointReturnsError(t *testing.T) {
	beforeTest()
	defer afterTest()

	var actualError error
	h, _ := New("hostname")
	actualError = h.HandleEndpoint("", "http://anotherhostname")
	if actualError == nil {
		t.Fatal("expected empty endpoint to return error")
	}
	if !strings.Contains(actualError.Error(), "path is empty") {
		t.Error("expected error to be empty path message")
	}

	actualError = h.HandleEndpoint("/foobar", "http://invalid.%2312.hostname")
	if actualError == nil {
		t.Fatal("expected invalid override hostname to return error")
	}
	if !strings.Contains(actualError.Error(), "invalid endpoint url") {
		t.Error("expected error to be invalid endpoint url message")
	}
}

func BenchmarkRouteHandling(b *testing.B) {
	beforeTest()
	defer afterTest()

	h, err := New("http://defaulthostname/")
	if err != nil {
		b.Fatal("unable to create proxy")
	}
	type route struct{ path, endpoint string }
	routes := [3]route{
		route{"/bazqux", "http://elsewhere.com"},
		route{"/foo", "http://cnn.com"},
		route{"/", "http://reddit.com"},
	}
	for _, r := range routes {
		err = h.HandleEndpoint(r.path, r.endpoint)
		if err != nil {
			b.Fatalf("unable to handle endpoint: %v -> %v", r.path, r.endpoint)
		}
	}

	endpointRequests := make(map[string]int)
	httpmock.RegisterNoResponder(func(r *http.Request) (*http.Response, error) {
		endpointRequests[r.URL.Host]++
		return httpmock.NewStringResponse(200, ""), nil
	})

	pathRequests := make(map[string]int)
	for i := 0; i < b.N; i++ {
		n := rand.Int() % len(routes)
		pathRequests[routes[n].path]++
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", routes[n].path, nil))
	}

	var result = []byte("\nPath     Requests     Hostname        Received Delta\n")
	for _, route := range routes {
		u, _ := url.Parse(route.endpoint)
		pathHits := pathRequests[route.path]
		endpointHits := endpointRequests[u.Host]
		result = append(result, fmt.Sprintf("% 8s  % 4v  %20s   % 4v    %04v  \n", route.path, pathHits, route.endpoint, endpointHits, pathHits-endpointHits)...)
		if pathHits != endpointHits {
			b.Errorf("Requests made to %s do not match requests received by %s\nExpected: %d\nActual %d\n", route.path, route.endpoint, pathHits, endpointHits)
		}
	}
	fmt.Println(string(result))
}
