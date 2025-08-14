package configurator

import "os"

// GetProxyEnvironment returns a map of proxy environment variables if HTTP_PROXY is set
func GetProxyEnvironment() map[string]string {
	proxy := make(map[string]string)
	if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		proxy["HTTP_PROXY"] = httpProxy
	}
	if httpsProxy := os.Getenv("HTTPS_PROXY"); httpsProxy != "" {
		proxy["HTTPS_PROXY"] = httpsProxy
	}
	if noProxy := os.Getenv("NO_PROXY"); noProxy != "" {
		proxy["NO_PROXY"] = noProxy
	}
	return proxy
}
