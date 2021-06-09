package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gorilla/mux"
	butil "github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/grpcutils"
	"github.com/wealdtech/go-bytesutil"
)

// DeserializeRequestBodyIntoContainer deserializes the request's body into an endpoint-specific struct.
func DeserializeRequestBodyIntoContainer(body io.Reader, requestContainer interface{}) ErrorJson {
	if err := json.NewDecoder(body).Decode(&requestContainer); err != nil {
		e := fmt.Errorf("could not decode request body: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// ProcessRequestContainerFields processes fields of an endpoint-specific container according to field tags.
func ProcessRequestContainerFields(requestContainer interface{}) ErrorJson {
	if err := processField(requestContainer, []fieldProcessor{
		{
			tag: "hex",
			f:   hexToBase64Processor,
		},
	}); err != nil {
		e := fmt.Errorf("could not process request data: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// SetRequestBodyToRequestContainer makes the endpoint-specific container the new body of the request.
func SetRequestBodyToRequestContainer(requestContainer interface{}, request *http.Request) ErrorJson {
	// Serialize the struct, which now includes a base64-encoded value, into JSON.
	j, err := json.Marshal(requestContainer)
	if err != nil {
		e := fmt.Errorf("could not marshal request: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	// Set the body to the new JSON.
	request.Body = ioutil.NopCloser(bytes.NewReader(j))
	request.Header.Set("Content-Length", strconv.Itoa(len(j)))
	request.ContentLength = int64(len(j))
	return nil
}

// PrepareRequestForProxying applies additional logic to the request so that it can be correctly proxied to grpc-gateway.
func (m *ApiProxyMiddleware) PrepareRequestForProxying(endpoint Endpoint, request *http.Request) ErrorJson {
	request.URL.Scheme = "http"
	request.URL.Host = m.GatewayAddress
	request.RequestURI = ""
	if errJson := HandleUrlParameters(endpoint.Path, request, endpoint.GetRequestUrlLiterals); errJson != nil {
		return errJson
	}
	return HandleQueryParameters(request, endpoint.GetRequestQueryParams)
}

// HandleUrlParameters processes URL parameters, allowing parameterized URLs to be safely and correctly proxied to grpc-gateway.
func HandleUrlParameters(url string, request *http.Request, literals []string) ErrorJson {
	segments := strings.Split(url, "/")

segmentsLoop:
	for i, s := range segments {
		// We only care about segments which are parameterized.
		if isRequestParam(s) {
			// Don't do anything with parameters which should be forwarded literally to gRPC.
			for _, l := range literals {
				if s == "{"+l+"}" {
					continue segmentsLoop
				}
			}

			routeVar := mux.Vars(request)[s[1:len(s)-1]]
			bRouteVar := []byte(routeVar)
			isHex, err := butil.IsHex(bRouteVar)
			if err != nil {
				e := fmt.Errorf("could not process URL parameter: %w", err)
				return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
			}
			if isHex {
				bRouteVar, err = bytesutil.FromHexString(string(bRouteVar))
				if err != nil {
					e := fmt.Errorf("could not process URL parameter: %w", err)
					return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
				}
			}
			// Converting hex to base64 may result in a value which malforms the URL.
			// We use URLEncoding to safely escape such values.
			base64RouteVar := base64.URLEncoding.EncodeToString(bRouteVar)

			// Merge segments back into the full URL.
			splitPath := strings.Split(request.URL.Path, "/")
			splitPath[i] = base64RouteVar
			request.URL.Path = strings.Join(splitPath, "/")
		}
	}
	return nil
}

// HandleQueryParameters processes query parameters, allowing them to be safely and correctly proxied to grpc-gateway.
func HandleQueryParameters(request *http.Request, params []QueryParam) ErrorJson {
	queryParams := request.URL.Query()

	for key, vals := range queryParams {
		for _, p := range params {
			if key == p.Name {
				if p.Hex {
					queryParams.Del(key)
					for _, v := range vals {
						b := []byte(v)
						isHex, err := butil.IsHex(b)
						if err != nil {
							e := fmt.Errorf("could not process query parameter: %w", err)
							return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
						}
						if isHex {
							b, err = bytesutil.FromHexString(v)
							if err != nil {
								e := fmt.Errorf("could not process query parameter: %w", err)
								return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
							}
						}
						queryParams.Add(key, base64.URLEncoding.EncodeToString(b))
					}
				}
				if p.Enum {
					queryParams.Del(key)
					for _, v := range vals {
						// gRPC expects uppercase enum values.
						queryParams.Add(key, strings.ToUpper(v))
					}
				}
			}
		}
	}
	request.URL.RawQuery = queryParams.Encode()
	return nil
}

// ProxyRequest proxies the request to grpc-gateway.
func ProxyRequest(request *http.Request) (*http.Response, ErrorJson) {
	grpcResp, err := http.DefaultClient.Do(request)
	if err != nil {
		e := fmt.Errorf("could not proxy request: %w", err)
		return nil, &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	if grpcResp == nil {
		return nil, &DefaultErrorJson{Message: "nil response from gRPC-gateway", Code: http.StatusInternalServerError}
	}
	return grpcResp, nil
}

// ReadGrpcResponseBody reads the body from the grpc-gateway's response.
func ReadGrpcResponseBody(reader io.Reader) ([]byte, ErrorJson) {
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		e := fmt.Errorf("could not read response body: %w", err)
		return nil, &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return body, nil
}

// DeserializeGrpcResponseBodyIntoErrorJson deserializes the body from the grpc-gateway's response into an error struct.
// The struct can be later examined to check if the request resulted in an error.
func DeserializeGrpcResponseBodyIntoErrorJson(errJson ErrorJson, body []byte) ErrorJson {
	if err := json.Unmarshal(body, errJson); err != nil {
		e := fmt.Errorf("could not unmarshal error: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// HandleGrpcResponseError acts on an error that resulted from a grpc-gateway's response.
func HandleGrpcResponseError(errJson ErrorJson, response *http.Response, writer http.ResponseWriter) {
	// Something went wrong, but the request completed, meaning we can write headers and the error message.
	for h, vs := range response.Header {
		for _, v := range vs {
			writer.Header().Set(h, v)
		}
	}
	// Set code to HTTP code because unmarshalled body contained gRPC code.
	errJson.SetCode(response.StatusCode)
	WriteError(writer, errJson, response.Header)
}

// GrpcResponseIsStatusCodeOnly checks whether a grpc-gateway's response contained no body.
func GrpcResponseIsStatusCodeOnly(request *http.Request, responseContainer interface{}) bool {
	return request.Method == "GET" && responseContainer == nil
}

// DeserializeGrpcResponseBodyIntoContainer deserializes the grpc-gateway's response body into an endpoint-specific struct.
func DeserializeGrpcResponseBodyIntoContainer(body []byte, responseContainer interface{}) ErrorJson {
	if err := json.Unmarshal(body, &responseContainer); err != nil {
		e := fmt.Errorf("could not unmarshal response: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// ProcessMiddlewareResponseFields processes fields of an endpoint-specific container according to field tags.
func ProcessMiddlewareResponseFields(responseContainer interface{}) ErrorJson {
	if err := processField(responseContainer, []fieldProcessor{
		{
			tag: "hex",
			f:   base64ToHexProcessor,
		},
		{
			tag: "enum",
			f:   enumToLowercaseProcessor,
		},
		{
			tag: "time",
			f:   timeToUnixProcessor,
		},
	}); err != nil {
		e := fmt.Errorf("could not process response data: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// SerializeMiddlewareResponseIntoJson serializes the endpoint-specific response struct into a JSON representation.
func SerializeMiddlewareResponseIntoJson(responseContainer interface{}) (jsonResponse []byte, errJson ErrorJson) {
	j, err := json.Marshal(responseContainer)
	if err != nil {
		e := fmt.Errorf("could not marshal response: %w", err)
		return nil, &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return j, nil
}

// WriteMiddlewareResponseHeadersAndBody populates headers and the body of the final response.
func WriteMiddlewareResponseHeadersAndBody(request *http.Request, grpcResponse *http.Response, responseJson []byte, writer http.ResponseWriter) ErrorJson {
	var statusCodeHeader string
	for h, vs := range grpcResponse.Header {
		// We don't want to expose any gRPC metadata in the HTTP response, so we skip forwarding metadata headers.
		if strings.HasPrefix(h, "Grpc-Metadata") {
			if h == "Grpc-Metadata-"+grpcutils.HttpCodeMetadataKey {
				statusCodeHeader = vs[0]
			}
		} else {
			for _, v := range vs {
				writer.Header().Set(h, v)
			}
		}
	}
	if request.Method == "GET" {
		writer.Header().Set("Content-Length", strconv.Itoa(len(responseJson)))
		if statusCodeHeader != "" {
			code, err := strconv.Atoi(statusCodeHeader)
			if err != nil {
				e := fmt.Errorf("could not parse status code: %w", err)
				return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
			}
			writer.WriteHeader(code)
		} else {
			writer.WriteHeader(grpcResponse.StatusCode)
		}
		if _, err := io.Copy(writer, ioutil.NopCloser(bytes.NewReader(responseJson))); err != nil {
			e := fmt.Errorf("could not write response message: %w", err)
			return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
		}
	} else if request.Method == "POST" {
		writer.WriteHeader(grpcResponse.StatusCode)
	}
	return nil
}

// WriteError writes the error by manipulating headers and the body of the final response.
func WriteError(writer http.ResponseWriter, errJson ErrorJson, responseHeader http.Header) {
	// Include custom error in the error JSON.
	if responseHeader != nil {
		customError, ok := responseHeader["Grpc-Metadata-"+grpcutils.CustomErrorMetadataKey]
		if ok {
			// Assume header has only one value and read the 0 index.
			if err := json.Unmarshal([]byte(customError[0]), errJson); err != nil {
				log.WithError(err).Error("Could not unmarshal custom error message")
				return
			}
		}
	}

	j, err := json.Marshal(errJson)
	if err != nil {
		log.WithError(err).Error("Could not marshal error message")
		return
	}

	writer.Header().Set("Content-Length", strconv.Itoa(len(j)))
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(errJson.StatusCode())
	if _, err := io.Copy(writer, ioutil.NopCloser(bytes.NewReader(j))); err != nil {
		log.WithError(err).Error("Could not write error message")
	}
}

// Cleanup performs final cleanup on the initial response from grpc-gateway.
func Cleanup(grpcResponseBody io.ReadCloser) ErrorJson {
	if err := grpcResponseBody.Close(); err != nil {
		e := fmt.Errorf("could not close response body: %w", err)
		return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
	}
	return nil
}

// isRequestParam verifies whether the passed string is a request parameter.
// Request parameters are enclosed in { and }.
func isRequestParam(s string) bool {
	return len(s) > 2 && s[0] == '{' && s[len(s)-1] == '}'
}

// processField calls each processor function on any field that has the matching tag set.
// It is a recursive function.
func processField(s interface{}, processors []fieldProcessor) error {
	kind := reflect.TypeOf(s).Kind()
	if kind != reflect.Ptr && kind != reflect.Slice && kind != reflect.Array {
		return fmt.Errorf("processing fields of kind '%v' is unsupported", kind)
	}

	t := reflect.TypeOf(s).Elem()
	v := reflect.Indirect(reflect.ValueOf(s))

	for i := 0; i < t.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Slice:
			sliceElem := t.Field(i).Type.Elem()
			kind := sliceElem.Kind()
			// Recursively process slices to struct pointers.
			if kind == reflect.Ptr && sliceElem.Elem().Kind() == reflect.Struct {
				for j := 0; j < v.Field(i).Len(); j++ {
					if err := processField(v.Field(i).Index(j).Interface(), processors); err != nil {
						return fmt.Errorf("could not process field '%s': %w", t.Field(i).Name, err)
					}
				}
			}
			// Process each string in string slices.
			if kind == reflect.String {
				for _, proc := range processors {
					_, hasTag := t.Field(i).Tag.Lookup(proc.tag)
					if hasTag {
						for j := 0; j < v.Field(i).Len(); j++ {
							if err := proc.f(v.Field(i).Index(j)); err != nil {
								return fmt.Errorf("could not process field '%s': %w", t.Field(i).Name, err)
							}
						}
					}
				}

			}
		// Recursively process struct pointers.
		case reflect.Ptr:
			if v.Field(i).Elem().Kind() == reflect.Struct {
				if err := processField(v.Field(i).Interface(), processors); err != nil {
					return fmt.Errorf("could not process field '%s': %w", t.Field(i).Name, err)
				}
			}
		default:
			field := t.Field(i)
			for _, proc := range processors {
				if _, hasTag := field.Tag.Lookup(proc.tag); hasTag {
					if err := proc.f(v.Field(i)); err != nil {
						return fmt.Errorf("could not process field '%s': %w", t.Field(i).Name, err)
					}
				}
			}
		}
	}
	return nil
}

func hexToBase64Processor(v reflect.Value) error {
	b, err := bytesutil.FromHexString(v.String())
	if err != nil {
		return err
	}
	v.SetString(base64.StdEncoding.EncodeToString(b))
	return nil
}

func base64ToHexProcessor(v reflect.Value) error {
	b, err := base64.StdEncoding.DecodeString(v.String())
	if err != nil {
		return err
	}
	v.SetString(hexutil.Encode(b))
	return nil
}

func enumToLowercaseProcessor(v reflect.Value) error {
	v.SetString(strings.ToLower(v.String()))
	return nil
}

func timeToUnixProcessor(v reflect.Value) error {
	t, err := time.Parse(time.RFC3339, v.String())
	if err != nil {
		return err
	}
	v.SetString(strconv.FormatUint(uint64(t.Unix()), 10))
	return nil
}