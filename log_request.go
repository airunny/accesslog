package accesslog

import (
	"bytes"
	"io"
)

type logReqBody interface {
	io.ReadCloser
	Body() []byte
}

type commonReqBody struct {
	body io.ReadCloser
	buf  *bytes.Buffer

	recordBody bool
}

func newLogReqBody(body io.ReadCloser, buf *bytes.Buffer, recordBody bool) logReqBody {
	return &commonReqBody{
		body:       body,
		buf:        buf,
		recordBody: recordBody,
	}
}

func (r *commonReqBody) Read(p []byte) (n int, err error) {
	n, err = r.body.Read(p)
	if r.recordBody && n > 0{
		bucket := r.buf.Cap() - r.buf.Len()
		if bucket >n{
			r.buf.Write(p[0:n])
		}else{
			r.buf.Write(p[0:bucket])
		}
	}
	return n, err
}

func (r *commonReqBody) Close() error {
	return r.body.Close()
}

func (r *commonReqBody) Body() []byte {
	return r.buf.Bytes()
}
