package wiremock

// mapping contains the mapping of requests to their configured responses
// http://wiremock.org/docs/request-matching/
type mapping struct {
	Request struct {
		URLPath string `json:"url,omitempty"`
		Method  string `json:"method,omitempty"`
		Headers *struct {
			Accept struct {
				Contains string `json:"contains,omitempty"`
			} `json:"Accept,omitempty"`
		} `json:"headers,omitempty"`
		QueryParameters *struct {
			SearchTerm struct {
				EqualTo string `json:"equalTo,omitempty"`
			} `json:"search_term,omitempty"`
		} `json:"queryParameters,omitempty"`
		Cookies *struct {
			Session struct {
				Matches string `json:"matches,omitempty"`
			} `json:"session,omitempty"`
		} `json:"cookies,omitempty"`
		BodyPatterns []struct {
			EqualToXML   string `json:"equalToXml,omitempty"`
			MatchesXPath string `json:"matchesXPath,omitempty"`
		} `json:"bodyPatterns,omitempty"`
		MultipartPatterns []struct {
			MatchingType string `json:"matchingType,omitempty"`
			Headers      struct {
				ContentDisposition struct {
					Contains string `json:"contains,omitempty"`
				} `json:"Content-Disposition,omitempty"`
				ContentType struct {
					Contains string `json:"contains,omitempty"`
				} `json:"Content-Type,omitempty"`
			} `json:"headers,omitempty"`
			BodyPatterns []struct {
				EqualToJSON string `json:"equalToJson,omitempty"`
			} `json:"bodyPatterns,omitempty"`
		} `json:"multipartPatterns,omitempty"`
		/*
			BasicAuthCredentials struct {
				Username string `json:"username,omitempty"`
				Password string `json:"password,omitempty"`
			} `json:"basicAuthCredentials,omitempty"`
		*/
	} `json:"request,omitempty"`
	Response struct {
		Status     int    `json:"status,omitempty"`
		Base64Body []byte `json:"base64Body,omitempty"`
		Headers    struct {
			ContentType string `json:"Content-Type"`
		} `json:"headers,omitempty"`
	} `json:"response,omitempty"`
}
