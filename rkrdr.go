package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/sirupsen/logrus"
)

type SyncBool struct {
	val bool
	m   *sync.Mutex
}

func newSyncBool(val bool) *SyncBool {
	return &SyncBool{val: val}
}
func (s *SyncBool) get() bool {
	s.m.Lock()
	defer s.m.Unlock()
	return s.val
}

func (s *SyncBool) set(val bool) {
	s.m.Lock()
	s.val = val
	s.m.Unlock()
}

type Rkrdr interface {
	Run() error
	Record(r *RecordInfo) error
	NextSeqNum() uint64
}

type rkrdr struct {
	userConn  net.Conn
	miniredis *miniredis.Miniredis

	seqNum    uint64
	closed    *SyncBool
	closeChan chan struct{}

	outFile *os.File
}

const NUM_RKRDR_CHILDREN = 4

func NewRkrdr(c net.Conn, m *miniredis.Miniredis) Rkrdr {
	return &rkrdr{
		userConn:  c,
		miniredis: m,

		closed:    newSyncBool(false),
		closeChan: make(chan struct{}, NUM_RKRDR_CHILDREN),
	}
}

func (r *rkrdr) Run() error {
	redisConn, err := net.Dial("tcp", r.miniredis.Addr())
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("rkrdr-%d.rkrdr", time.Now().Unix())
	if r.outFile, err = os.Create(fileName); err != nil {
		return err
	}

	toRedisPR, toRedisPW := net.Pipe()
	toRedis := io.MultiWriter(redisConn, toRedisPR)
	go func() {
		for {
			if _, err := io.Copy(toRedis, r.userConn); err != nil {
				logrus.Info(err)
			}
		}
		r.closed.set(true)
		r.closeChan <- struct{}{}
	}()

	toUserPR, toUserPW := net.Pipe()
	toUser := io.MultiWriter(r.userConn, toUserPR)
	go func() {
		for {
			if _, err := io.Copy(toUser, redisConn); err != nil {
				logrus.Info(err)
			}
			fmt.Println("yolo")
		}
		r.closed.set(true)
		r.closeChan <- struct{}{}
	}()

	addr := r.userConn.RemoteAddr().String()
	go func() {
		recordContent(toRedisPW, addr, true, r)
		r.closed.set(true)
		r.closeChan <- struct{}{}
	}()
	go func() {
		recordContent(toUserPW, addr, false, r)
		r.closed.set(true)
		r.closeChan <- struct{}{}
	}()
	return nil
}

func (r *rkrdr) NextSeqNum() uint64 {
	return atomic.AddUint64(&r.seqNum, 1)
}

const RKRDR_CLOSE_TIMEOUT = time.Second * 5

var ErrRkrdrCloseTimeout = errors.New("rkrdr timed out waiting to close all children")

func (r *rkrdr) Close() error {
	r.closed.set(true)
	for i := 0; i < 4; i++ {
		select {
		case _ = <-r.closeChan:
		case _ = <-time.After(RKRDR_CLOSE_TIMEOUT):
			return ErrRkrdrCloseTimeout
		}
	}

	return r.outFile.Close()
}

func (r *rkrdr) Record(record *RecordInfo) error {
	_, err := r.outFile.WriteString(record.String() + "\n")
	if err != nil {
		return err
	}
	return r.outFile.Sync()
}

func recordContent(r io.Reader, addr string, isRedisClient bool, rkrdr Rkrdr) {
	bufReader := bufio.NewReader(r)
	prefix := fmt.Sprintf("%s: %s", addr, "  to redis")
	direction := "to"
	if !isRedisClient {
		prefix = fmt.Sprintf("%s: %s", addr, "from redis")
		direction = "from"
	}

	logrus.Infof("[%s] rkrdr begun\n", prefix)

	for {
		ra, err := readString(bufReader)
		if err != nil {
			logrus.Error(err)
			logrus.Errorf("[%s] exiting rkrdr\n", prefix)
			return
		}
		recordInfo := RecordInfo{
			Ctr:  rkrdr.NextSeqNum(),
			Addr: addr,
			Dir:  direction,
			Msg:  ra,
		}
		logrus.Infof("[%s] %q\n", prefix, recordInfo.String())
		rkrdr.Record(&recordInfo)

	}
}
