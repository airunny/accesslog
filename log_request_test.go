package accesslog

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestCommonLogRespWriter_Write(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0,18))
	read := bytes.NewBufferString("hello,world,golang00")
	body := ioutil.NopCloser(read)

	reqBody := newLogReqBody(body,buf,true)

	p := make([]byte,20,20)
	n,err :=reqBody.Read(p)
	if err != nil {
		t.Error(err)
		return
	}

	if n != 20 {
		t.Errorf("expected %v but got %v",10,n)
		return
	}

	if bytes.Equal(p, buf.Bytes()) {
		t.Errorf("%v not equal %v",p,buf.Bytes())
		return
	}

	 if !bytes.Equal(reqBody.Body(),[]byte("hello,world,golang"))  {
		t.Errorf("expectec hello,world,golang but got %v",reqBody.Body())
	}
}
