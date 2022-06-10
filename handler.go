package accesslog

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	logBufMax  = 1 << 10 // 1KB
	bodyBufMax = 1 << 10 // 1KB
)

var (
	logging Logger
)

func Handler(h http.Handler, opts ...Option) http.Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var err error
	if o.log != nil {
		logging = o.log
	} else if o.filename != "" {
		logging, err = newAsyncFileLogger(o.filename)
		if err != nil {
			panic(err)
		}
	} else {
		logging = newGlogLogger()
	}

	return &handler{
		handler: h,
		logging: logging,
		opts:    o,
		logBufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, logBufMax))
			},
		},
		bodyBufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, bodyBufMax))
			},
		},
	}
}

func Flush() error {
	if logging != nil {
		return logging.Close()
	}
	return nil
}

type handler struct {
	handler     http.Handler
	logging     Logger
	opts        *options
	logBufPool  sync.Pool
	bodyBufPool sync.Pool
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()

	// wrap req body
	reqBodyBuf := h.bodyBufPool.Get().(*bytes.Buffer)
	reqBodyBuf.Reset()
	defer h.bodyBufPool.Put(reqBodyBuf)
	reqBody := newLogReqBody(req.Body, reqBodyBuf, h.opts.requestBody && canRecordBody(req.Header))
	req.Body = reqBody

	// wrap ResponseWriter
	respBodyBuf := h.bodyBufPool.Get().(*bytes.Buffer)
	respBodyBuf.Reset()
	defer h.bodyBufPool.Put(respBodyBuf)

	wrapResponse := newResponseWriter(w, respBodyBuf, h.opts.responseBody)
	h.handler.ServeHTTP(wrapResponse, req)
	logBuf := h.fmtLog(req, *req.URL, start, reqBody, wrapResponse)
	h.logging.Log(logBuf.Bytes())
}

func (h *handler) fmtLog(req *http.Request, u url.URL, start time.Time, wrapRequestBody logReqBody, wrapResponse logResponseWriter) *bytes.Buffer {
	elapsed := time.Now().Sub(start)
	buf := h.logBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	// now
	buf.WriteString(strconv.FormatInt(start.UnixNano(), 10))
	buf.WriteByte('\t')

	// method
	buf.WriteString(req.Method)
	buf.WriteByte('\t')

	// uri
	buf.WriteString(u.RequestURI())
	buf.WriteByte('\t')

	// req header
	buf.WriteByte('{')
	buf.WriteString(fmtHeader("Content-Length", req.ContentLength))
	buf.WriteByte(',')
	buf.WriteString(fmtHeader("Host", req.Host))
	buf.WriteByte(',')
	buf.WriteString(fmtHeader("IP", req.RemoteAddr))
	kvs, sorter := sortedKeyValues(req.Header)
	for _, kv := range kvs {
		if len(kv.values) > 0 {
			buf.WriteByte(',')
			buf.WriteString(fmtHeader(http.CanonicalHeaderKey(kv.key), kv.values[0]))
		}
	}
	headerSorterPool.Put(sorter)
	buf.WriteByte('}')
	buf.WriteByte('\t')

	// req body
	reqBodySize := len(wrapRequestBody.Body())
	if reqBodySize > 0 {
		if req.ContentLength != int64(reqBodySize) {
			buf.WriteString("{too large to display}")
		} else {
			buf.Write(wrapRequestBody.Body())
		}
	} else {
		buf.WriteString("{no data}")
	}
	buf.WriteByte('\t')

	// status
	buf.WriteString(strconv.FormatInt(int64(wrapResponse.StatusCode()), 10))
	buf.WriteByte('\t')

	// resp header
	buf.WriteByte('{')
	kvs, sorter = sortedKeyValues(wrapResponse.Header())
	for i, kv := range kvs {
		if len(kv.values) > 0 {
			if i != 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(fmtHeader(http.CanonicalHeaderKey(kv.key), kv.values[0]))
		}
	}
	headerSorterPool.Put(sorter)
	buf.WriteByte('}')
	buf.WriteByte('\t')

	// resp body
	respBodySize := len(wrapResponse.Body())
	if respBodySize > 0 {
		if wrapResponse.Size() != respBodySize {
			buf.WriteString("{too large to display}")
		} else {
			if wrapResponse.Body()[respBodySize-1] == '\n' {
				buf.Write(wrapResponse.Body()[:respBodySize-1])
			} else {
				buf.Write(wrapResponse.Body())
			}
		}

	} else {
		buf.WriteString("{no data}")
	}
	buf.WriteByte('\t')

	// content-length
	buf.WriteString(strconv.FormatInt(int64(wrapResponse.Size()), 10))
	buf.WriteByte('\t')

	// elapsed time
	buf.WriteString(strconv.FormatInt(int64(elapsed/time.Microsecond), 10))
	buf.WriteByte('\n')

	return buf
}

func fmtHeader(key string, value interface{}) string {
	return fmt.Sprintf(`"%v":"%v"`, key, value)
}

func canRecordBody(header http.Header) bool {
	ct := header.Get("Content-type")
	if i := strings.IndexByte(ct, ';'); i != -1 {
		ct = ct[:i]
	}
	switch ct {
	case "application/json":
		return true
	case "application/x-www-form-urlencoded":
		return true
	case "application/xml":
		return true
	case "text/plain":
		return true
	case "text/xml":
		return true
	default:
		return false
	}
}
