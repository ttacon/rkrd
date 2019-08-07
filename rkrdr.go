package main

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/alicebob/miniredis"
	"github.com/sirupsen/logrus"
)

type Rkrdr interface {
	Run() error
}

type rkrdr struct {
	userConn  net.Conn
	miniredis *miniredis.Miniredis
}

func NewRkrdr(c net.Conn, m *miniredis.Miniredis) Rkrdr {
	return &rkrdr{
		userConn:  c,
		miniredis: m,
	}
}

func (r *rkrdr) Run() error {
	fmt.Println(r)
	fmt.Println(r.miniredis)
	redisConn, err := net.Dial("tcp", r.miniredis.Addr())
	if err != nil {
		return err
	}

	toRedisPR, toRedisPW := net.Pipe()
	toRedis := io.MultiWriter(redisConn, toRedisPR)
	go func() {
		for {
			io.Copy(toRedis, r.userConn)
		}
	}()

	toUserPR, toUserPW := net.Pipe()
	toUser := io.MultiWriter(r.userConn, toUserPR)
	go func() {
		for {
			io.Copy(toUser, redisConn)
		}
	}()

	addr := r.userConn.RemoteAddr().String()
	go recordContent(toRedisPW, addr, true)
	go recordContent(toUserPW, addr, false)
	return nil
}

func recordContent(r io.Reader, addr string, isRedisClient bool) {
	bufReader := bufio.NewReader(r)
	prefix := fmt.Sprintf("%s: %s", addr, "  to redis")
	if !isRedisClient {
		prefix = fmt.Sprintf("%s: %s", addr, "from redis")
	}

	logrus.Infof("[%s] rkrdr begun\n", prefix)

	for {
		ra, err := readString(bufReader)
		if err != nil {
			logrus.Error(err)
			logrus.Errorf("[%s] exiting rkrdr\n", prefix)
			return
		}
		logrus.Infof("[%s] %q\n", prefix, ra)
	}
}
