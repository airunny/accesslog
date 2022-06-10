package accesslog

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestHandler(t *testing.T) {
	ast := assert.New(t)

	filename := fmt.Sprintf("%saccesslog%d.log", os.TempDir(), time.Now().Unix())
	defer os.Remove(filename)
	ts := httptest.NewServer(Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.Write([]byte(`{"name": "peter", "age": 12}`))
	}), WithFileName(filename)))
	defer ts.Close()

	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)

	resp, err = http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)

	resp, err = http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)
	Flush()

	f, err := os.Open(filename)
	ast.Nil(err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := make([]string, 0, 3)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	ast.Nil(err)

	ast.Equal(3, len(lines))
	checkLine(ast, lines[0], `{"user": "admin"}`, `{"name": "peter", "age": 12}`)
	checkLine(ast, lines[1], `{"user": "admin"}`, `{"name": "peter", "age": 12}`)
	checkLine(ast, lines[2], `{"user": "admin"}`, `{"name": "peter", "age": 12}`)

}

func checkLine(ast *assert.Assertions, line, req, resp string) {
	arr := strings.Split(line, "\t")

	ast.Equal(arr[4], req)
	ast.Equal(arr[7], resp)
}

func buildConfig() zap.Config {
	var config zap.Config
	config = zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("01/02 15:04:05")
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	return config
}

func buildLogger() (*zap.Logger, error) {
	return buildConfig().Build()
}

type sugaredLogger struct {
	sugared *zap.SugaredLogger
}

func (s *sugaredLogger) Log(bytes []byte) error {
	s.sugared.Info(string(bytes))
	return nil
}

func (s *sugaredLogger) Close() error {
	return s.sugared.Sync()
}

func (s *sugaredLogger) QueueBufferSize() int {
	return 0
}

func TestLoggerHandler(t *testing.T) {
	ast := assert.New(t)
	logger, err := buildLogger()
	ast.Nil(err)

	ts := httptest.NewServer(Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		w.Header().Add("Content-type", "application/json")
		w.Write([]byte(`{"name": "peter", "age": 12}`))
	}), WithLogger(&sugaredLogger{sugared: logger.Sugar()})))
	defer ts.Close()

	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)

	resp, err = http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)

	resp, err = http.Post(ts.URL, "application/json", strings.NewReader(`{"user": "admin"}`))
	ast.Nil(err)
	resp.Body.Close()
	ast.Equal(200, resp.StatusCode)
	Flush()
}
