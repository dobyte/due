package http

import "net/http"

const (
	StatusContinue           = http.StatusContinue           // RFC 9110, 15.2.1
	StatusSwitchingProtocols = http.StatusSwitchingProtocols // RFC 9110, 15.2.2
	StatusProcessing         = http.StatusProcessing         // RFC 2518, 10.1
	StatusEarlyHints         = http.StatusEarlyHints         // RFC 8297

	StatusOK                   = http.StatusOK                   // RFC 9110, 15.3.1
	StatusCreated              = http.StatusCreated              // RFC 9110, 15.3.2
	StatusAccepted             = http.StatusAccepted             // RFC 9110, 15.3.3
	StatusNonAuthoritativeInfo = http.StatusNonAuthoritativeInfo // RFC 9110, 15.3.4
	StatusNoContent            = http.StatusNoContent            // RFC 9110, 15.3.5
	StatusResetContent         = http.StatusResetContent         // RFC 9110, 15.3.6
	StatusPartialContent       = http.StatusPartialContent       // RFC 9110, 15.3.7
	StatusMultiStatus          = http.StatusMultiStatus          // RFC 4918, 11.1
	StatusAlreadyReported      = http.StatusAlreadyReported      // RFC 5842, 7.1
	StatusIMUsed               = http.StatusIMUsed               // RFC 3229, 10.4.1

	StatusMultipleChoices   = http.StatusMultipleChoices   // RFC 9110, 15.4.1
	StatusMovedPermanently  = http.StatusMovedPermanently  // RFC 9110, 15.4.2
	StatusFound             = http.StatusFound             // RFC 9110, 15.4.3
	StatusSeeOther          = http.StatusSeeOther          // RFC 9110, 15.4.4
	StatusNotModified       = http.StatusNotModified       // RFC 9110, 15.4.5
	StatusUseProxy          = http.StatusUseProxy          // RFC 9110, 15.4.6
	StatusTemporaryRedirect = http.StatusTemporaryRedirect // RFC 9110, 15.4.8
	StatusPermanentRedirect = http.StatusPermanentRedirect // RFC 9110, 15.4.9

	StatusBadRequest                   = http.StatusBadRequest                   // RFC 9110, 15.5.1
	StatusUnauthorized                 = http.StatusUnauthorized                 // RFC 9110, 15.5.2
	StatusPaymentRequired              = http.StatusPaymentRequired              // RFC 9110, 15.5.3
	StatusForbidden                    = http.StatusForbidden                    // RFC 9110, 15.5.4
	StatusNotFound                     = http.StatusNotFound                     // RFC 9110, 15.5.5
	StatusMethodNotAllowed             = http.StatusMethodNotAllowed             // RFC 9110, 15.5.6
	StatusNotAcceptable                = http.StatusNotAcceptable                // RFC 9110, 15.5.7
	StatusProxyAuthRequired            = http.StatusProxyAuthRequired            // RFC 9110, 15.5.8
	StatusRequestTimeout               = http.StatusRequestTimeout               // RFC 9110, 15.5.9
	StatusConflict                     = http.StatusConflict                     // RFC 9110, 15.5.10
	StatusGone                         = http.StatusGone                         // RFC 9110, 15.5.11
	StatusLengthRequired               = http.StatusLengthRequired               // RFC 9110, 15.5.12
	StatusPreconditionFailed           = http.StatusPreconditionFailed           // RFC 9110, 15.5.13
	StatusRequestEntityTooLarge        = http.StatusRequestEntityTooLarge        // RFC 9110, 15.5.14
	StatusRequestURITooLong            = http.StatusRequestURITooLong            // RFC 9110, 15.5.15
	StatusUnsupportedMediaType         = http.StatusUnsupportedMediaType         // RFC 9110, 15.5.16
	StatusRequestedRangeNotSatisfiable = http.StatusRequestedRangeNotSatisfiable // RFC 9110, 15.5.17
	StatusExpectationFailed            = http.StatusExpectationFailed            // RFC 9110, 15.5.18
	StatusTeapot                       = http.StatusTeapot                       // RFC 9110, 15.5.19 (Unused)
	StatusMisdirectedRequest           = http.StatusMisdirectedRequest           // RFC 9110, 15.5.20
	StatusUnprocessableEntity          = http.StatusUnprocessableEntity          // RFC 9110, 15.5.21
	StatusLocked                       = http.StatusLocked                       // RFC 4918, 11.3
	StatusFailedDependency             = http.StatusFailedDependency             // RFC 4918, 11.4
	StatusTooEarly                     = http.StatusTooEarly                     // RFC 8470, 5.2.
	StatusUpgradeRequired              = http.StatusUpgradeRequired              // RFC 9110, 15.5.22
	StatusPreconditionRequired         = http.StatusPreconditionRequired         // RFC 6585, 3
	StatusTooManyRequests              = http.StatusTooManyRequests              // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge  = http.StatusRequestHeaderFieldsTooLarge  // RFC 6585, 5
	StatusUnavailableForLegalReasons   = http.StatusUnavailableForLegalReasons   // RFC 7725, 3

	StatusInternalServerError           = http.StatusInternalServerError           // RFC 9110, 15.6.1
	StatusNotImplemented                = http.StatusNotImplemented                // RFC 9110, 15.6.2
	StatusBadGateway                    = http.StatusBadGateway                    // RFC 9110, 15.6.3
	StatusServiceUnavailable            = http.StatusServiceUnavailable            // RFC 9110, 15.6.4
	StatusGatewayTimeout                = http.StatusGatewayTimeout                // RFC 9110, 15.6.5
	StatusHTTPVersionNotSupported       = http.StatusHTTPVersionNotSupported       // RFC 9110, 15.6.6
	StatusVariantAlsoNegotiates         = http.StatusVariantAlsoNegotiates         // RFC 2295, 8.1
	StatusInsufficientStorage           = http.StatusInsufficientStorage           // RFC 4918, 11.5
	StatusLoopDetected                  = http.StatusLoopDetected                  // RFC 5842, 7.2
	StatusNotExtended                   = http.StatusNotExtended                   // RFC 2774, 7
	StatusNetworkAuthenticationRequired = http.StatusNetworkAuthenticationRequired // RFC 6585, 6
)

func StatusText(code int) string {
	return http.StatusText(code)
}
