package accesslog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/golang/glog"
)

// MaxLogFileSize the max log size
const MaxLogFileSize int64 = 1024 * 1024 * 1800

type Logger interface {
	Log([]byte) error
	Close() error
	QueueBufferSize() int
}

type asyncFileLogger struct {
	filename string
	file     *os.File
	queue    goconcurrentqueue.Queue
	close    chan struct{}
	sizeNum  int64
}

func newAsyncFileLogger(filename string) (Logger, error) {
	dir, _ := filepath.Split(filename)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	f, err := openAppendFile(filename)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	ret := &asyncFileLogger{
		filename: filename,
		file:     f,
		queue:    goconcurrentqueue.NewFIFO(),
		close:    make(chan struct{}),
		sizeNum:  stat.Size(),
	}

	go ret.loop()

	return ret, nil
}

func openAppendFile(fileName string) (*os.File, error) {
	return os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModeAppend|0666)
}

func (l *asyncFileLogger) Log(data []byte) error {
	return l.queue.Enqueue(data)
}

func (l *asyncFileLogger) loop() {
	for {
		select {
		case <-l.close:
			return
		default:
		}

		data, err := l.queue.Dequeue()
		if err != nil {
			time.Sleep(time.Millisecond * 10)
			continue
		}
		l.writeFile(data.([]byte))
	}
}

func (l *asyncFileLogger) writeFile(data []byte) {
	if int64(len(data))+l.sizeNum > MaxLogFileSize {
		l.rotateLog()
	}

	n, err := l.file.Write(data)
	if err != nil {
		panic("cannot write access log")
	}

	l.sizeNum += int64(n)
}

func (l *asyncFileLogger) rotateLog() {
	_ = l.file.Sync()
	_ = l.file.Close()
	err := os.Rename(l.filename, fmt.Sprintf("%s-%s", l.filename, time.Now().Format("20060102150405")))
	if err != nil {
		panic("fail to rotate log")
	}

	l.file, err = openAppendFile(l.filename)
	if err != nil {
		panic(err)
	}

	stat, err := l.file.Stat()
	if err != nil {
		panic(err)
	}

	l.sizeNum = stat.Size()
}

func (l *asyncFileLogger) QueueBufferSize() int {
	return l.queue.GetLen()
}

func (l *asyncFileLogger) Close() error {
	l.close <- struct{}{}

	for {
		data, err := l.queue.Dequeue()
		if err != nil {
			goto Done
		}
		l.writeFile(data.([]byte))
	}

Done:
	l.file.Sync()
	return l.file.Close()
}

// GlogLogger glog
type GlogLogger struct{}

func newGlogLogger() Logger {
	return GlogLogger{}
}

func (g GlogLogger) Log(data []byte) error {
	glog.Info(string(data))
	return nil
}

func (g GlogLogger) Close() error {
	glog.Flush()
	return nil
}

func (g GlogLogger) QueueBufferSize() int {
	return 0
}
