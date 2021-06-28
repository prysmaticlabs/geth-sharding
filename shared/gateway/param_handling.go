package gateway

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	butil "github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/wealdtech/go-bytesutil"
)

// HandleURLParameters processes URL parameters, allowing parameterized URLs to be safely and correctly proxied to grpc-gateway.
func HandleURLParameters(url string, req *http.Request, literals []string) ErrorJson {
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

			routeVar := mux.Vars(req)[s[1:len(s)-1]]
			bRouteVar := []byte(routeVar)
			isHex, err := butil.IsHex(bRouteVar)
			if err != nil {
				e := errors.Wrapf(err, "could not process URL parameter")
				return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
			}
			if isHex {
				bRouteVar, err = bytesutil.FromHexString(string(bRouteVar))
				if err != nil {
					e := errors.Wrapf(err, "could not process URL parameter")
					return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
				}
			}
			// Converting hex to base64 may result in a value which malforms the URL.
			// We use URLEncoding to safely escape such values.
			base64RouteVar := base64.URLEncoding.EncodeToString(bRouteVar)

			// Merge segments back into the full URL.
			splitPath := strings.Split(req.URL.Path, "/")
			splitPath[i] = base64RouteVar
			req.URL.Path = strings.Join(splitPath, "/")
		}
	}
	return nil
}

// HandleQueryParameters processes query parameters, allowing them to be safely and correctly proxied to grpc-gateway.
func HandleQueryParameters(req *http.Request, params []QueryParam) ErrorJson {
	queryParams := req.URL.Query()

	for key, vals := range queryParams {
		for _, p := range params {
			if key == p.Name {
				if p.Hex {
					queryParams.Del(key)
					for _, v := range vals {
						b := []byte(v)
						isHex, err := butil.IsHex(b)
						if err != nil {
							e := errors.Wrapf(err, "could not process query parameter")
							return &DefaultErrorJson{Message: e.Error(), Code: http.StatusInternalServerError}
						}
						if isHex {
							b, err = bytesutil.FromHexString(v)
							if err != nil {
								e := errors.Wrapf(err, "could not process query parameter")
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
	req.URL.RawQuery = queryParams.Encode()
	return nil
}

// isRequestParam verifies whether the passed string is a request parameter.
// Request parameters are enclosed in { and }.
func isRequestParam(s string) bool {
	return len(s) > 2 && s[0] == '{' && s[len(s)-1] == '}'
}
