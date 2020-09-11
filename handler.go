package accesslog

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	logBufMax  = 1 << 10 // 1KB
	bodyBufMax = 1 << 10 // 1KB
)

var (
	logBufPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, logBufMax))
		},
	}

	bodyBufPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, bodyBufMax))
		},
	}

	logging  logger
	reqBody  int32 = 0
	respBody int32 = 0
)

type Conf struct {
	Filename     string `json:"filename"`
	RequestBody  bool   `json:"request_body"`
	ResponseBody bool   `json:"response_body"`
}

// switch recording request body at runtime
func SwitchReqBody(b bool) {
	if b {
		atomic.StoreInt32(&reqBody, 1)
	} else {
		atomic.StoreInt32(&reqBody, 0)
	}
}

// switch recording response body at runtime
func SwitchRespBody(b bool) {
	if b {
		atomic.StoreInt32(&respBody, 1)
	} else {
		atomic.StoreInt32(&respBody, 0)
	}
}

func SetLogging(log logger) {
	logging = log
}

func Handler(h http.Handler, opts ...Option) http.Handler {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	var err error
	if options.cfg != nil {
		logging, err = newAsyncFileLogger(options.cfg)
		if err != nil {
			panic(err)
		}

		if options.cfg.RequestBody {
			reqBody = 1
		}

		if options.cfg.ResponseBody {
			respBody = 1
		}
	} else {
		logging = newGlogLogger()
		reqBody = 1
		respBody = 1
	}

	return &handler{
		handler: h,
	}
}

func Flush() error {
	if logging != nil {
		return logging.Close()
	}
	return nil
}

type HealthStat struct {
	LoggerBufferSize int
}

func FetchHealthStat() HealthStat {
	return HealthStat{
		LoggerBufferSize: logging.QueueBufferSize(),
	}
}

type handler struct {
	handler http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()

	// wrap req body
	reqBodyBuf := bodyBufPool.Get().(*bytes.Buffer)
	reqBodyBuf.Reset()
	defer bodyBufPool.Put(reqBodyBuf)
	reqBody := newLogReqBody(req.Body, reqBodyBuf, atomic.LoadInt32(&reqBody) == 1 && canRecordBody(req.Header))
	req.Body = reqBody

	// wrap ResponseWriter
	respBodyBuf := bodyBufPool.Get().(*bytes.Buffer)
	respBodyBuf.Reset()
	defer bodyBufPool.Put(respBodyBuf)

	wrapResponse := newResponseWriter(w, respBodyBuf, atomic.LoadInt32(&respBody) == 1)
	h.handler.ServeHTTP(wrapResponse, req)
	logBuf := fmtLog(req, *req.URL, start, reqBody, wrapResponse)
	logging.Log(logBuf)
}

func fmtLog(req *http.Request, u url.URL, start time.Time, wrapRequestBody logReqBody, wrapResponse logResponseWriter) *bytes.Buffer {
	elapsed := time.Now().Sub(start)
	buf := logBufPool.Get().(*bytes.Buffer)
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
