package runner

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/sw33tLie/http"
	"github.com/sw33tLie/http/httptrace"
	"github.com/sw33tLie/http/httputil"

	"github.com/sw33tLie/uff/v2/pkg/constants"
	"github.com/sw33tLie/uff/v2/pkg/features"
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

// SimpleRunner handles HTTP requests for fuzzing
type SimpleRunner struct {
	config      *ffuf.Config
	client      *http.Client
	preHooks    []ffuf.PreRequestHook
	postHooks   []ffuf.PostResponseHook
	rateLimiter *features.AdaptiveLimiter
}

// AddPreHook adds a hook to be executed before the request
func (runner *SimpleRunner) AddPreHook(hook ffuf.PreRequestHook) {
	runner.preHooks = append(runner.preHooks, hook)
}

// AddPostHook adds a hook to be executed after the response
func (runner *SimpleRunner) AddPostHook(hook ffuf.PostResponseHook) {
	runner.postHooks = append(runner.postHooks, hook)
}

// NewSimpleRunner creates a new SimpleRunner instance
func NewSimpleRunner(conf *ffuf.Config, replay bool) ffuf.RunnerProvider {
	runner := &SimpleRunner{
		config: conf,
	}
	runner.initClient(conf, replay)
	runner.registerHooks(conf)
	return runner
}

func (runner *SimpleRunner) initClient(conf *ffuf.Config, replay bool) {
	proxyURL := http.ProxyFromEnvironment
	customProxy := conf.ProxyURL
	if replay {
		customProxy = conf.ReplayProxyURL
	}

	if len(customProxy) > 0 {
		if parsedProxy, err := url.Parse(customProxy); err == nil {
			proxyURL = http.ProxyURL(parsedProxy)
		}
	}

	cert := runner.loadCertificates(conf)

	runner.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       time.Duration(conf.Timeout) * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2:   conf.Http2,
			Proxy:               proxyURL,
			MaxIdleConns:        constants.MaxIdleConns,
			MaxIdleConnsPerHost: constants.MaxIdleConnsPerHost,
			MaxConnsPerHost:     constants.MaxConnsPerHost,
			DialContext: (&net.Dialer{
				Timeout: time.Duration(conf.Timeout) * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: time.Duration(conf.Timeout) * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: conf.InsecureSSL,
				MinVersion:         tls.VersionTLS10,
				Renegotiation:      tls.RenegotiateOnceAsClient,
				ServerName:         conf.SNI,
				Certificates:       cert,
			},
		},
	}

	if conf.FollowRedirects {
		runner.client.CheckRedirect = runner.checkRedirect
	}
}

func (runner *SimpleRunner) loadCertificates(conf *ffuf.Config) []tls.Certificate {
	if conf.ClientCert != "" && conf.ClientKey != "" {
		if cert, err := tls.LoadX509KeyPair(conf.ClientCert, conf.ClientKey); err == nil {
			return []tls.Certificate{cert}
		}
	}
	return []tls.Certificate{}
}

func (runner *SimpleRunner) registerHooks(conf *ffuf.Config) {
	if conf.AutoTune {
		runner.rateLimiter = features.NewAdaptiveLimiter()
	}
	if conf.LFI {
		runner.AddPostHook(features.NewLFIHook())
	}
	if conf.DiscoverBackup {
		hook, err := features.NewBackupHookWithStatusCodesList(conf.BackupLevel, conf.BackupStatusCodes)
		if err != nil {
			// Log warning but continue (backup discovery is optional)
			slog.Warn("Failed to initialize backup hook", "error", err)
		} else {
			runner.AddPostHook(hook)
		}
	}
	if conf.Spider {
		runner.AddPostHook(features.NewSpiderHook())
	}
	// Always enable soft error detection or potentially make it configurable
	runner.AddPostHook(features.NewSoftErrorHook())
}

// checkRedirect implements a custom redirect policy
func (runner *SimpleRunner) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= ffuf.MAX_REDIRECTS_FOLLOW {
		return fmt.Errorf("stopped after %d redirects", ffuf.MAX_REDIRECTS_FOLLOW)
	}
	// Future improvement: Loop detection logic here
	return nil
}

// Prepare replaces keywords in the request with input values
func (runner *SimpleRunner) Prepare(input map[string][]byte, baseRequest *ffuf.Request) (ffuf.Request, error) {
	req := ffuf.CopyRequest(baseRequest)

	// Replace keywords (twice to handle nested replacements or complex cases)
	for iteration := 0; iteration < ffuf.KEYWORD_REPLACEMENT_ITERATIONS; iteration++ {
		for keyword, inputItem := range input {
			itemString := string(inputItem)
			req.Method = strings.ReplaceAll(req.Method, keyword, itemString)
			req.Url = strings.ReplaceAll(req.Url, keyword, itemString)
			req.Opaque = strings.ReplaceAll(req.Opaque, keyword, itemString)
			req.Data = []byte(strings.ReplaceAll(string(req.Data), keyword, itemString))

			// Handle headers
			headers := make(map[string]string, len(req.Headers))
			for headerName, headerValue := range req.Headers {
				headers[strings.ReplaceAll(headerName, keyword, itemString)] = strings.ReplaceAll(headerValue, keyword, itemString)
			}
			req.Headers = headers
		}
	}

	if runner.config.MethodAsRawRequest {
		req.Method = strings.TrimRight(req.Method, "\r\n") + "\r\n\r\n"
	}

	req.Input = input
	return req, nil
}

// Execute performs the HTTP request
func (runner *SimpleRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	if err := runner.runPreHooks(req); err != nil {
		return ffuf.Response{}, err
	}

	rawMethod := runner.prepareRawMethod(req.Method)
	httpReq, err := runner.createHTTPRequest(req, rawMethod)
	if err != nil {
		return ffuf.Response{}, err
	}

	return runner.performRequest(httpReq, req, rawMethod)
}

func (runner *SimpleRunner) runPreHooks(req *ffuf.Request) error {
	for _, hook := range runner.preHooks {
		if err := hook.Execute(req); err != nil {
			return err
		}
	}
	return nil
}

func (runner *SimpleRunner) prepareRawMethod(method string) string {
	if runner.config.MethodAsRawRequest {
		return strings.TrimRight(method, "\r\n\t ") + "\r\n\r\n"
	}
	return method
}

// createHTTPRequest builds the http.Request object
func (runner *SimpleRunner) createHTTPRequest(req *ffuf.Request, rawMethod string) (*http.Request, error) {
	var httpReq *http.Request
	var err error
	data := bytes.NewReader(req.Data)

	// Proxy compatibility for raw requests
	if runner.config.MethodAsRawRequest && (len(runner.config.ProxyURL) > 0 || len(runner.config.ReplayProxyURL) > 0) {
		return runner.parseRawRequestForProxy(rawMethod, req.Url)
	}

	// Standard request creation
	if runner.config.MethodAsRawRequest {
		http.EnableMethodOnlyRequest()
	}

	httpReq, err = http.NewRequestWithContext(runner.config.Context, rawMethod, req.Url, data)
	if err != nil {
		return nil, err
	}

	return runner.configureRequestHeaders(httpReq, req), nil
}

func (runner *SimpleRunner) parseRawRequestForProxy(rawMethod, targetURL string) (*http.Request, error) {
	parsedReq, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawMethod)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse raw request: %w", err)
	}

	parsedReq.RequestURI = ""
	if parsedUrl, err := url.Parse(targetURL); err == nil {
		parsedReq.URL.Scheme = parsedUrl.Scheme
		parsedReq.URL.Host = parsedUrl.Host
	}
	return parsedReq, nil
}

func (runner *SimpleRunner) configureRequestHeaders(httpReq *http.Request, req *ffuf.Request) *http.Request {
	// Set User-Agent
	if runner.config.RandomAgent {
		req.Headers["User-Agent"] = features.GetRandomUserAgent()
	} else if _, ok := req.Headers["User-Agent"]; !ok {
		req.Headers["User-Agent"] = constants.DefaultUserAgent
	}

	// Set Host
	if host, ok := req.Headers["Host"]; ok {
		httpReq.Host = host
	}

	// Opaque
	if runner.config.Raw {
		httpReq.URL.Opaque = req.Url
	}
	if req.Opaque != "" {
		httpReq.URL.Opaque = req.Opaque
	}

	// Set all headers
	for headerName, headerValue := range req.Headers {
		httpReq.Header.Set(headerName, headerValue)
	}
	return httpReq
}

// performRequest executes the request and handles the response
func (runner *SimpleRunner) performRequest(httpReq *http.Request, req *ffuf.Request, rawMethod string) (ffuf.Response, error) {
	var rawReq []byte

	// Prepare trace for timing
	var start time.Time
	var firstByteTime time.Duration
	trace := &httptrace.ClientTrace{
		WroteRequest:         func(info httptrace.WroteRequestInfo) { start = time.Now() },
		GotFirstResponseByte: func() { firstByteTime = time.Since(start) },
	}
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(runner.config.Context, trace))

	// Dump request if needed
	if len(runner.config.OutputDirectory) > 0 || len(runner.config.AuditLog) > 0 {
		rawReq, _ = httputil.DumpRequestOut(httpReq, true)
		req.Raw = string(rawReq)
	}

	if runner.config.DoNotSendContentLength {
		http.DoNotSendContentLength()
	}

	httpResp, err := runner.client.Do(httpReq)
	if err != nil {
		return ffuf.Response{}, err
	}
	defer httpResp.Body.Close()

	runner.applyRateLimit(httpResp.StatusCode)
	req.Timestamp = start

	return runner.createResponse(httpResp, req, firstByteTime, rawReq)
}

func (runner *SimpleRunner) applyRateLimit(statusCode int) {
	if runner.config.AutoTune && runner.rateLimiter != nil {
		if delay := runner.rateLimiter.Adjust(statusCode); delay > 0 {
			select {
			case <-runner.config.Context.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

func (runner *SimpleRunner) createResponse(httpResp *http.Response, req *ffuf.Request, duration time.Duration, rawReq []byte) (ffuf.Response, error) {
	resp := ffuf.NewResponse(httpResp, req)

	// Content Length and Auto-skip
	if sizeString := httpResp.Header.Get("Content-Length"); sizeString != "" {
		if size, err := strconv.Atoi(sizeString); err == nil {
			resp.ContentLength = int64(size)
			if runner.config.IgnoreBody || size > constants.MaxDownloadSize {
				resp.Cancelled = true
				return resp, nil
			}
		}
	}

	// Dump response if needed
	if len(runner.config.OutputDirectory) > 0 || len(runner.config.AuditLog) > 0 {
		rawResp, _ := httputil.DumpResponse(httpResp, true)
		resp.Request.Raw = string(rawReq)
		resp.Raw = string(rawResp)
	}

	// Read Body
	bodyBytes, err := runner.readResponseBody(httpResp)
	if err == nil {
		resp.ContentLength = int64(len(bodyBytes))
		resp.Data = bodyBytes
	}

	// Calc words/lines
	resp.ContentWords = int64(ffuf.CountWords(resp.Data))
	resp.ContentLines = int64(ffuf.CountLines(resp.Data))
	resp.Duration = duration
	resp.Timestamp = req.Timestamp.Add(duration)

	// Hooks
	for _, hook := range runner.postHooks {
		if hookErr := hook.Execute(&resp, req); hookErr != nil {
			slog.Warn("Post-hook error", "hook", hook.Name(), "error", hookErr)
		}
	}

	return resp, nil
}

func (runner *SimpleRunner) readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.ReadCloser = resp.Body

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		if gzipReader, err := gzip.NewReader(resp.Body); err == nil {
			reader = gzipReader
		}
	case "br":
		reader = io.NopCloser(brotli.NewReader(resp.Body))
	case "deflate":
		reader = flate.NewReader(resp.Body)
	}
	defer reader.Close()

	// Limit read size to prevent memory exhaustion from large responses
	maxSize := int64(constants.MaxDownloadSize)
	limitedReader := io.LimitReader(reader, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return data[:maxSize], nil
	}
	return data, nil
}

// Dump dumps the request for debugging/logging
func (runner *SimpleRunner) Dump(req *ffuf.Request) ([]byte, error) {
	rawMethod := runner.prepareRawMethod(req.Method)
	httpReq, err := runner.createHTTPRequest(req, rawMethod)
	if err != nil {
		return nil, err
	}
	return httputil.DumpRequestOut(httpReq, true)
}
